package extractor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const (
	extractorAPIURL = "https://api.anthropic.com/v1/messages"
	extractorModel  = "claude-sonnet-4-6"
	extractorMaxRetries = 3
	extractorTimeout    = 60 * time.Second
)

type claudeExtractor struct {
	apiKey     string
	httpClient *http.Client
}

func newClaudeExtractor() *claudeExtractor {
	return &claudeExtractor{
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{},
	}
}

// --- API types (minimal — extraction only, no tool use) ---

type extractorContentBlock struct {
	Type   string          `json:"type"`
	Source *extractorSource `json:"source,omitempty"`
	Text   string          `json:"text,omitempty"`
}

type extractorSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type extractorMessage struct {
	Role    string                  `json:"role"`
	Content []extractorContentBlock `json:"content"`
}

type extractorRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []extractorMessage `json:"messages"`
}

type extractorResponseBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type extractorResponse struct {
	Content    []extractorResponseBlock `json:"content"`
	StopReason string                   `json:"stop_reason"`
}

func (c *claudeExtractor) ExtractTaxDocument(ctx context.Context, fileBytes []byte, documentType string) (*models.TaxDocument, error) {
	prompt := extractionPrompt(documentType)
	if prompt == "" {
		return nil, fmt.Errorf("claude extractor: unsupported document type %q", documentType)
	}

	encoded := base64.StdEncoding.EncodeToString(fileBytes)

	msg := extractorMessage{
		Role: "user",
		Content: []extractorContentBlock{
			{
				Type: "document",
				Source: &extractorSource{
					Type:      "base64",
					MediaType: "application/pdf",
					Data:      encoded,
				},
			},
			{Type: "text", Text: prompt},
		},
	}

	reqBody := extractorRequest{
		Model:     extractorModel,
		MaxTokens: 512,
		Messages:  []extractorMessage{msg},
	}

	var lastErr error
	for attempt := 1; attempt <= extractorMaxRetries; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt) * 5 * time.Second
			log.Printf("[extractor] retry attempt %d/%d — backing off %s", attempt, extractorMaxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, fmt.Errorf("claude extractor: context cancelled during backoff: %w", ctx.Err())
			}
		}

		doc, err := c.doExtraction(ctx, reqBody, documentType)
		if err == nil {
			return doc, nil
		}
		lastErr = err
		log.Printf("[extractor] attempt %d failed: %v", attempt, err)
	}
	return nil, fmt.Errorf("claude extractor: all %d attempts failed: %w", extractorMaxRetries, lastErr)
}

func (c *claudeExtractor) doExtraction(ctx context.Context, reqBody extractorRequest, documentType string) (*models.TaxDocument, error) {
	callCtx, cancel := context.WithTimeout(context.Background(), extractorTimeout)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-callCtx.Done():
		}
	}()

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(callCtx, "POST", extractorAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[extractor] HTTP %d: %s", resp.StatusCode, rawBody)
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, rawBody)
	}

	var apiResp extractorResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return nil, fmt.Errorf("envelope parse: %w", err)
	}

	for _, block := range apiResp.Content {
		if block.Type != "text" {
			continue
		}
		text := strings.TrimSpace(block.Text)
		fields, err := parseExtractionJSON(text)
		if err != nil {
			return nil, fmt.Errorf("parse fields: %w (raw: %.300s)", err, text)
		}

		taxYear := parseTaxYear(fields["tax_year"])
		docType := models.DocumentType(documentType)

		log.Printf("[extractor] extracted %d fields for %s tax_year=%d", len(fields), documentType, taxYear)
		return &models.TaxDocument{
			DocumentType: docType,
			TaxYear:      taxYear,
			Fields:       fields,
			UploadedAt:   time.Now(),
		}, nil
	}
	return nil, fmt.Errorf("no text block in response")
}

// extractionPrompt returns a type-specific extraction instruction.
func extractionPrompt(documentType string) string {
	switch documentType {
	case "w2":
		return `Extract the following fields from this W-2 tax form and return ONLY a raw JSON object with no markdown, no code fences, no text before or after.

Required fields: employer_name, gross_wages, federal_withheld, state_withheld, social_security_wages, tax_year

Example: {"employer_name":"Acme Corp","gross_wages":"95000","federal_withheld":"14200","state_withheld":"5700","social_security_wages":"95000","tax_year":"2024"}

If a field is not present on the form, omit it. Return only numeric values for dollar amounts (no $ sign, no commas).`

	case "1099":
		return `Extract the following fields from this 1099 tax form and return ONLY a raw JSON object with no markdown, no code fences, no text before or after.

Required fields: payer_name, gross_income, federal_withheld, income_type, tax_year

For income_type, use one of: nec (non-employee compensation), div (dividends), int (interest), misc (miscellaneous).

Example: {"payer_name":"Stripe Inc","gross_income":"12000","federal_withheld":"0","income_type":"nec","tax_year":"2024"}

If a field is not present on the form, omit it. Return only numeric values for dollar amounts (no $ sign, no commas).`

	case "1098":
		return `Extract the following fields from this 1098 mortgage interest statement and return ONLY a raw JSON object with no markdown, no code fences, no text before or after.

Required fields: lender_name, mortgage_interest_paid, points_paid, outstanding_principal, tax_year

Example: {"lender_name":"Chase Bank","mortgage_interest_paid":"18500","points_paid":"0","outstanding_principal":"380000","tax_year":"2024"}

If a field is not present on the form, omit it. Return only numeric values for dollar amounts (no $ sign, no commas).`

	default:
		return ""
	}
}

func parseExtractionJSON(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl != -1 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		s = s[start : end+1]
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(s), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func parseTaxYear(s string) int {
	if len(s) == 4 {
		year := 0
		for _, ch := range s {
			if ch < '0' || ch > '9' {
				return 0
			}
			year = year*10 + int(ch-'0')
		}
		return year
	}
	return 0
}
