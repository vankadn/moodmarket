// infrastructure/auth/clerk_auth.go
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

const clerkAPIBase = "https://api.clerk.com/v1"

// ClerkAuthProvider validates Clerk session JWTs without any Clerk SDK.
// It verifies the RS256 signature using Clerk's JWKS endpoint, then fetches
// user details from the Clerk backend API. Both are cached in memory.
type ClerkAuthProvider struct {
	secretKey  string
	loginURL   string
	httpClient *http.Client

	// JWKS cache: kid → RSA public key. Populated on first call, never evicted
	// (Clerk rotates keys rarely; a restart picks up new keys automatically).
	jwksMu sync.RWMutex
	jwks   map[string]*rsa.PublicKey

	// User cache: userID → identity. Avoids a Clerk API call on every request.
	// TTL-less by design for Phase 4; acceptable since user data rarely changes.
	userMu    sync.RWMutex
	userCache map[string]*ports.UserIdentity
}

func NewClerkAuthProvider(secretKey string) *ClerkAuthProvider {
	return &ClerkAuthProvider{
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		jwks:       make(map[string]*rsa.PublicKey),
		userCache:  make(map[string]*ports.UserIdentity),
	}
}

func (p *ClerkAuthProvider) GetLoginURL() string {
	return p.loginURL
}

// ValidateToken verifies the Clerk session JWT and returns the caller's identity.
// Flow: parse JWT → check expiry → verify RS256 signature via JWKS → fetch user details.
func (p *ClerkAuthProvider) ValidateToken(ctx context.Context, token string) (*ports.UserIdentity, error) {
	if token == "" {
		return nil, fmt.Errorf("clerk: missing bearer token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("clerk: malformed JWT (expected 3 parts, got %d)", len(parts))
	}

	// Decode and validate header.
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("clerk: decode header: %w", err)
	}
	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("clerk: parse header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("clerk: unexpected alg %q — expected RS256", header.Alg)
	}
	if header.Kid == "" {
		return nil, fmt.Errorf("clerk: missing kid in header")
	}

	// Decode and validate claims.
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("clerk: decode payload: %w", err)
	}
	var claims struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
		Nbf int64  `json:"nbf"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("clerk: parse claims: %w", err)
	}
	now := time.Now().Unix()
	if claims.Exp > 0 && now > claims.Exp {
		return nil, fmt.Errorf("clerk: token expired")
	}
	if claims.Nbf > 0 && now < claims.Nbf {
		return nil, fmt.Errorf("clerk: token not yet valid")
	}
	if claims.Sub == "" {
		return nil, fmt.Errorf("clerk: missing sub claim")
	}

	// Verify RS256 signature: sha256(header.payload) signed with Clerk's private key.
	pubKey, err := p.publicKey(ctx, header.Kid)
	if err != nil {
		return nil, fmt.Errorf("clerk: get public key: %w", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("clerk: decode signature: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sig); err != nil {
		return nil, fmt.Errorf("clerk: invalid signature")
	}

	return p.user(ctx, claims.Sub)
}

// publicKey returns the RSA public key for the given kid, fetching JWKS from Clerk if needed.
func (p *ClerkAuthProvider) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	p.jwksMu.RLock()
	key, ok := p.jwks[kid]
	p.jwksMu.RUnlock()
	if ok {
		return key, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", clerkAPIBase+"/jwks", nil)
	if err != nil {
		return nil, fmt.Errorf("build JWKS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read JWKS response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS API %d: %s", resp.StatusCode, body)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parse JWKS: %w", err)
	}

	p.jwksMu.Lock()
	for _, k := range jwks.Keys {
		pub, err := jwkToRSA(k.N, k.E)
		if err != nil {
			continue
		}
		p.jwks[k.Kid] = pub
	}
	p.jwksMu.Unlock()

	p.jwksMu.RLock()
	key, ok = p.jwks[kid]
	p.jwksMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("kid %q not found in Clerk JWKS", kid)
	}
	return key, nil
}

// user returns the UserIdentity for a Clerk user ID, fetching from the API if not cached.
func (p *ClerkAuthProvider) user(ctx context.Context, userID string) (*ports.UserIdentity, error) {
	p.userMu.RLock()
	if identity, ok := p.userCache[userID]; ok {
		p.userMu.RUnlock()
		return identity, nil
	}
	p.userMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, "GET", clerkAPIBase+"/users/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("build user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read user response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user API %d: %s", resp.StatusCode, body)
	}

	var clerkUser struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		EmailAddresses []struct {
			ID           string `json:"id"`
			EmailAddress string `json:"email_address"`
		} `json:"email_addresses"`
		PrimaryEmailAddressID string `json:"primary_email_address_id"`
	}
	if err := json.Unmarshal(body, &clerkUser); err != nil {
		return nil, fmt.Errorf("parse user response: %w", err)
	}

	var email string
	for _, ea := range clerkUser.EmailAddresses {
		if ea.ID == clerkUser.PrimaryEmailAddressID {
			email = ea.EmailAddress
			break
		}
	}

	identity := &ports.UserIdentity{
		UserID: clerkUser.ID,
		Email:  email,
		Name:   strings.TrimSpace(clerkUser.FirstName + " " + clerkUser.LastName),
	}

	p.userMu.Lock()
	p.userCache[userID] = identity
	p.userMu.Unlock()

	return identity, nil
}

// jwkToRSA converts a JWK's base64url-encoded modulus and exponent into an rsa.PublicKey.
func jwkToRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
