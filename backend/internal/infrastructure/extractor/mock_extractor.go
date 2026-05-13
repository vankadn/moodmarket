package extractor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockExtractor struct{}

func newMockExtractor() (*mockExtractor, error) {
	if os.Getenv("DEV_MODE") != "true" {
		return nil, fmt.Errorf("extractor factory: mock is not allowed in production (DEV_MODE != true)")
	}
	return &mockExtractor{}, nil
}

func (m *mockExtractor) ExtractTaxDocument(_ context.Context, _ []byte, documentType string) (*models.TaxDocument, error) {
	switch documentType {
	case "w2":
		return &models.TaxDocument{
			DocumentType: models.DocumentTypeW2,
			TaxYear:      2024,
			Fields: map[string]string{
				"employer_name":        "Acme Corp",
				"gross_wages":          "95000",
				"federal_withheld":     "14200",
				"state_withheld":       "5700",
				"social_security_wages": "95000",
				"tax_year":             "2024",
			},
			UploadedAt: time.Now(),
		}, nil
	case "1099":
		return &models.TaxDocument{
			DocumentType: models.DocumentType1099,
			TaxYear:      2024,
			Fields: map[string]string{
				"payer_name":       "Stripe Inc",
				"gross_income":     "12000",
				"federal_withheld": "0",
				"income_type":      "nec",
				"tax_year":         "2024",
			},
			UploadedAt: time.Now(),
		}, nil
	case "1098":
		return &models.TaxDocument{
			DocumentType: models.DocumentType1098,
			TaxYear:      2024,
			Fields: map[string]string{
				"lender_name":            "Chase Bank",
				"mortgage_interest_paid": "18500",
				"points_paid":            "0",
				"outstanding_principal":  "380000",
				"tax_year":               "2024",
			},
			UploadedAt: time.Now(),
		}, nil
	default:
		return nil, fmt.Errorf("mock extractor: unsupported document type %q", documentType)
	}
}
