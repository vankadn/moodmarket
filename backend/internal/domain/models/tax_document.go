package models

import "time"

type DocumentType string

const (
	DocumentTypeW2   DocumentType = "w2"
	DocumentType1099 DocumentType = "1099"
	DocumentType1098 DocumentType = "1098"
)

type TaxDocument struct {
	ID           string
	UserID       string
	DocumentType DocumentType
	TaxYear      int
	Fields       map[string]string
	Verified     bool
	UploadedAt   time.Time
	VerifiedAt   *time.Time
}
