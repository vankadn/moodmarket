package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type DocumentExtractor interface {
	ExtractTaxDocument(ctx context.Context, fileBytes []byte, documentType string) (*models.TaxDocument, error)
}
