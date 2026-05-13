package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type DocumentService struct {
	extractor    ports.DocumentExtractor
	documentRepo ports.DocumentRepository
}

func NewDocumentService(extractor ports.DocumentExtractor, documentRepo ports.DocumentRepository) *DocumentService {
	return &DocumentService{
		extractor:    extractor,
		documentRepo: documentRepo,
	}
}

// UploadDocument extracts structured fields from the PDF bytes and persists them.
// The PDF bytes are never stored — only the extracted fields map is saved.
func (s *DocumentService) UploadDocument(ctx context.Context, userID string, fileBytes []byte, docType string) (*models.TaxDocument, error) {
	log.Printf("[document] extract start: user=%s type=%s bytes=%d", userID, docType, len(fileBytes))

	doc, err := s.extractor.ExtractTaxDocument(ctx, fileBytes, docType)
	if err != nil {
		return nil, fmt.Errorf("document service: extract: %w", err)
	}

	doc.UserID = userID
	doc.UploadedAt = time.Now()

	if err := s.documentRepo.Save(ctx, doc); err != nil {
		return nil, fmt.Errorf("document service: save: %w", err)
	}

	log.Printf("[document] saved: user=%s type=%s tax_year=%d fields=%d id=%s", userID, docType, doc.TaxYear, len(doc.Fields), doc.ID)
	return doc, nil
}

func (s *DocumentService) ListDocuments(ctx context.Context, userID string) ([]*models.TaxDocument, error) {
	docs, err := s.documentRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("document service: list: %w", err)
	}
	return docs, nil
}

func (s *DocumentService) DeleteDocument(ctx context.Context, userID, docID string) error {
	if err := s.documentRepo.DeleteByID(ctx, userID, docID); err != nil {
		return fmt.Errorf("document service: delete: %w", err)
	}
	log.Printf("[document] deleted: user=%s doc=%s", userID, docID)
	return nil
}
