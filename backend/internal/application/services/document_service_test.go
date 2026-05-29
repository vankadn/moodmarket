// application/services/document_service_test.go
//
// DocumentService is a thin orchestration layer over extraction and storage.
// The contract worth pinning: extracted docs are stamped with userID + upload
// time before being saved, the raw PDF bytes are forwarded to the extractor,
// errors from either dependency are wrapped (not swallowed), and delete passes
// the userID through so a user can only delete their own document.
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

func TestUploadDocument_StampsAndSaves(t *testing.T) {
	t.Parallel()

	ext := &fakeExtractor{doc: &models.TaxDocument{TaxYear: 2024, Fields: map[string]string{"wages": "100"}}}
	repo := newFakeDocumentRepo()
	svc := NewDocumentService(ext, repo)

	pdf := []byte("%PDF-1.4 fake")
	got, err := svc.UploadDocument(context.Background(), "user-1", pdf, "W2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Raw bytes and type are forwarded to the extractor untouched.
	if string(ext.gotBytes) != string(pdf) || ext.gotType != "W2" {
		t.Errorf("extractor got bytes=%q type=%q, want pdf/W2", ext.gotBytes, ext.gotType)
	}
	// userID and UploadedAt are stamped before saving.
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", got.UserID)
	}
	if got.UploadedAt.IsZero() {
		t.Error("UploadedAt was not stamped")
	}
	if repo.saved == nil || repo.saved.UserID != "user-1" {
		t.Errorf("document was not saved with userID, saved=%+v", repo.saved)
	}
}

func TestUploadDocument_ExtractError(t *testing.T) {
	t.Parallel()

	ext := &fakeExtractor{err: errors.New("claude refused")}
	repo := newFakeDocumentRepo()
	svc := NewDocumentService(ext, repo)

	_, err := svc.UploadDocument(context.Background(), "user-1", []byte("x"), "W2")
	if err == nil {
		t.Fatal("expected extract error to propagate")
	}
	if repo.saved != nil {
		t.Error("nothing should be saved when extraction fails")
	}
}

func TestUploadDocument_SaveError(t *testing.T) {
	t.Parallel()

	ext := &fakeExtractor{doc: &models.TaxDocument{TaxYear: 2024}}
	repo := newFakeDocumentRepo()
	repo.saveErr = errors.New("mongo write failed")
	svc := NewDocumentService(ext, repo)

	if _, err := svc.UploadDocument(context.Background(), "user-1", []byte("x"), "W2"); err == nil {
		t.Fatal("expected save error to propagate")
	}
}

func TestListDocuments(t *testing.T) {
	t.Parallel()

	t.Run("returns_docs", func(t *testing.T) {
		t.Parallel()
		repo := newFakeDocumentRepo()
		repo.docs = []*models.TaxDocument{{ID: "1"}, {ID: "2"}}
		svc := NewDocumentService(&fakeExtractor{}, repo)

		got, err := svc.ListDocuments(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 docs, got %d", len(got))
		}
	})

	t.Run("propagates_error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeDocumentRepo()
		repo.listErr = errors.New("read failed")
		svc := NewDocumentService(&fakeExtractor{}, repo)

		if _, err := svc.ListDocuments(context.Background(), "user-1"); err == nil {
			t.Fatal("expected list error to propagate")
		}
	})
}

func TestDeleteDocument(t *testing.T) {
	t.Parallel()

	t.Run("passes_user_and_doc_id", func(t *testing.T) {
		t.Parallel()
		repo := newFakeDocumentRepo()
		svc := NewDocumentService(&fakeExtractor{}, repo)

		if err := svc.DeleteDocument(context.Background(), "user-1", "doc-9"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Scoping the delete to the userID is what stops one user deleting another's document.
		if repo.deletedUser != "user-1" || repo.deletedID != "doc-9" {
			t.Errorf("delete got user=%q id=%q, want user-1/doc-9", repo.deletedUser, repo.deletedID)
		}
	})

	t.Run("propagates_error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeDocumentRepo()
		repo.deleteErr = errors.New("not found")
		svc := NewDocumentService(&fakeExtractor{}, repo)

		if err := svc.DeleteDocument(context.Background(), "user-1", "doc-9"); err == nil {
			t.Fatal("expected delete error to propagate")
		}
	})
}
