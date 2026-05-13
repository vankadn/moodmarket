package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type documentUseCase interface {
	UploadDocument(ctx context.Context, userID string, fileBytes []byte, docType string) (*models.TaxDocument, error)
	ListDocuments(ctx context.Context, userID string) ([]*models.TaxDocument, error)
	DeleteDocument(ctx context.Context, userID, docID string) error
}

type DocumentHandler struct {
	service  documentUseCase
	identity ports.IdentityProvider
}

func NewDocumentHandler(svc documentUseCase, identity ports.IdentityProvider) *DocumentHandler {
	return &DocumentHandler{service: svc, identity: identity}
}

// ServeHTTP routes /documents/upload, /documents, and /documents/:id.
func (h *DocumentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch {
	case r.URL.Path == "/documents/upload" && r.Method == http.MethodPost:
		h.handleUpload(w, r, userID)
	case r.URL.Path == "/documents" && r.Method == http.MethodGet:
		h.handleList(w, r, userID)
	case strings.HasPrefix(r.URL.Path, "/documents/") && r.Method == http.MethodDelete:
		docID := strings.TrimPrefix(r.URL.Path, "/documents/")
		h.handleDelete(w, r, userID, docID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *DocumentHandler) handleUpload(w http.ResponseWriter, r *http.Request, userID string) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "request too large or invalid multipart form", http.StatusBadRequest)
		return
	}

	docType := strings.ToLower(strings.TrimSpace(r.FormValue("type")))
	if docType != "w2" && docType != "1099" && docType != "1098" {
		http.Error(w, `invalid type: must be "w2", "1099", or "1098"`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		http.Error(w, "missing document field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/pdf") && !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		http.Error(w, "only PDF files are accepted", http.StatusBadRequest)
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("document handler: read file: %v", err)
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	doc, err := h.service.UploadDocument(ctx, userID, fileBytes, docType)
	if err != nil {
		log.Printf("document handler: upload: %v", err)
		http.Error(w, "failed to extract document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func (h *DocumentHandler) handleList(w http.ResponseWriter, r *http.Request, userID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	docs, err := h.service.ListDocuments(ctx, userID)
	if err != nil {
		log.Printf("document handler: list: %v", err)
		http.Error(w, "failed to list documents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs)
}

func (h *DocumentHandler) handleDelete(w http.ResponseWriter, r *http.Request, userID, docID string) {
	if docID == "" {
		http.Error(w, "missing document id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.service.DeleteDocument(ctx, userID, docID); err != nil {
		log.Printf("document handler: delete: %v", err)
		http.Error(w, "failed to delete document", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
