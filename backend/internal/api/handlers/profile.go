package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type ProfileHandler struct {
	repo     ports.ProfileRepository
	identity ports.IdentityProvider
}

func NewProfileHandler(repo ports.ProfileRepository, identity ports.IdentityProvider) *ProfileHandler {
	return &ProfileHandler{repo: repo, identity: identity}
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.getProfile(w, r)
	case http.MethodPost:
		h.saveProfile(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ProfileHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := h.repo.GetByUserID(r.Context(), userID)
	if errors.Is(err, ports.ErrProfileNotFound) {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("profile handler: get: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

func (h *ProfileHandler) saveProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var profile models.UserProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	profile.UserID = userID

	if err := h.repo.Upsert(r.Context(), &profile); err != nil {
		log.Printf("profile handler: upsert: %v", err)
		http.Error(w, "failed to save profile", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)
}
