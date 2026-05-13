package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// taxDocumentBSON is the MongoDB storage shape. BSON tags are isolated here,
// never on domain models.
type taxDocumentBSON struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	UserID       string             `bson:"user_id"`
	DocumentType string             `bson:"document_type"`
	TaxYear      int                `bson:"tax_year"`
	Fields       map[string]string  `bson:"fields"`
	Verified     bool               `bson:"verified"`
	UploadedAt   time.Time          `bson:"uploaded_at"`
	VerifiedAt   *time.Time         `bson:"verified_at,omitempty"`
}

type MongoDocumentRepository struct {
	collection *mongo.Collection
}

func NewMongoDocumentRepository(db *mongo.Database) *MongoDocumentRepository {
	return &MongoDocumentRepository{collection: db.Collection("tax_documents")}
}

// Save upserts the document using a form-specific key:
//   - 1098: (user_id, document_type) — one per user, always replaces
//   - W2:   (user_id, document_type, tax_year, fields.employer_name) — per employer/year
//   - 1099: (user_id, document_type, tax_year, fields.payer_name)    — per payer/year
func (r *MongoDocumentRepository) Save(ctx context.Context, doc *models.TaxDocument) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := upsertFilter(doc)
	bsonDoc := fromTaxDocument(doc)
	bsonDoc.ID = primitive.ObjectID{} // let MongoDB generate / preserve the _id on upsert

	result, err := r.collection.ReplaceOne(ctx, filter, bsonDoc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongo document repo: save: %w", err)
	}
	if result.UpsertedID != nil {
		doc.ID = result.UpsertedID.(primitive.ObjectID).Hex()
	}
	return nil
}

func (r *MongoDocumentRepository) GetByUserID(ctx context.Context, userID string) ([]*models.TaxDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "uploaded_at", Value: -1}})
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo document repo: get by user: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []taxDocumentBSON
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo document repo: decode: %w", err)
	}

	result := make([]*models.TaxDocument, len(docs))
	for i, d := range docs {
		result[i] = toTaxDocument(&d)
	}
	return result, nil
}

func (r *MongoDocumentRepository) GetByID(ctx context.Context, id string) (*models.TaxDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("mongo document repo: invalid id %q: %w", id, err)
	}

	var doc taxDocumentBSON
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("mongo document repo: not found: %s", id)
		}
		return nil, fmt.Errorf("mongo document repo: get by id: %w", err)
	}
	return toTaxDocument(&doc), nil
}

func (r *MongoDocumentRepository) DeleteByID(ctx context.Context, userID, docID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(docID)
	if err != nil {
		return fmt.Errorf("mongo document repo: invalid id %q: %w", docID, err)
	}

	// Include user_id in the filter so users can only delete their own documents.
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return fmt.Errorf("mongo document repo: delete: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("mongo document repo: not found or not owned: %s", docID)
	}
	return nil
}

// upsertFilter returns the appropriate key for each document type.
func upsertFilter(doc *models.TaxDocument) bson.M {
	switch doc.DocumentType {
	case models.DocumentType1098:
		return bson.M{
			"user_id":       doc.UserID,
			"document_type": string(doc.DocumentType),
		}
	case models.DocumentTypeW2:
		return bson.M{
			"user_id":                doc.UserID,
			"document_type":          string(doc.DocumentType),
			"tax_year":               doc.TaxYear,
			"fields.employer_name":   doc.Fields["employer_name"],
		}
	default: // 1099
		return bson.M{
			"user_id":             doc.UserID,
			"document_type":       string(doc.DocumentType),
			"tax_year":            doc.TaxYear,
			"fields.payer_name":   doc.Fields["payer_name"],
		}
	}
}

// --- Conversion helpers ---

func fromTaxDocument(d *models.TaxDocument) *taxDocumentBSON {
	return &taxDocumentBSON{
		UserID:       d.UserID,
		DocumentType: string(d.DocumentType),
		TaxYear:      d.TaxYear,
		Fields:       d.Fields,
		Verified:     d.Verified,
		UploadedAt:   d.UploadedAt,
		VerifiedAt:   d.VerifiedAt,
	}
}

func toTaxDocument(doc *taxDocumentBSON) *models.TaxDocument {
	return &models.TaxDocument{
		ID:           doc.ID.Hex(),
		UserID:       doc.UserID,
		DocumentType: models.DocumentType(doc.DocumentType),
		TaxYear:      doc.TaxYear,
		Fields:       doc.Fields,
		Verified:     doc.Verified,
		UploadedAt:   doc.UploadedAt,
		VerifiedAt:   doc.VerifiedAt,
	}
}
