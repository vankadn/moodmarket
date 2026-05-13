package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type DocumentRepository interface {
	// Save upserts using form-specific keys:
	// 1098 → keyed on (user_id, document_type) — one per user, new upload replaces old
	// W2   → keyed on (user_id, document_type, tax_year, employer_name) — per employer per year
	// 1099 → keyed on (user_id, document_type, tax_year, payer_name)    — per payer per year
	Save(ctx context.Context, doc *models.TaxDocument) error
	GetByUserID(ctx context.Context, userID string) ([]*models.TaxDocument, error)
	GetByID(ctx context.Context, id string) (*models.TaxDocument, error)
	DeleteByID(ctx context.Context, userID, docID string) error
}
