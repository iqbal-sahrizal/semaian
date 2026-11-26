package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// JWKSVerifierConfig configures the production-oriented OIDC/JWT verifier.
type JWKSVerifierConfig struct {
	Issuer          string
	Audience        string
	JWKSURL         string
	HTTPTimeout     time.Duration
	CacheTTL        time.Duration
	RefreshCooldown time.Duration
	AllowInsecure   bool // Only for local/test mock JWKS servers
}

// JWKSVerifier validates JWTs using a remote JWKS endpoint with bounded caching,
// key rotation support, asymmetric algorithm pinning, and SSRF protections.
type JWKSVerifier struct {
	cfg     JWKSVerifierConfig
	client  *http.Client
	sfGroup singleflight.Group

	mu                 sync.RWMutex
	keys               map[string]crypto.PublicKey
	fetchedAt          time.Time
	lastRefreshAttempt time.Time
}

// NewJWKSVerifier creates a validated JWKSVerifier.
func NewJWKSVerifier(cfg JWKSVerifierConfig) (*JWKSVerifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" || cfg.JWKSURL == "" {
		return nil, errors.New("issuer, audience, and jwks_url are required")
	}
	if err := validateEndpointURL(cfg.JWKSURL, cfg.AllowInsecure); err != nil {
		return nil, fmt.Errorf("invalid jwks_url: %w", err)
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.RefreshCooldown <= 0 {
		cfg.RefreshCooldown = 15 * time.Second
	}

	client := &http.Client{
		Timeout: cfg.HTTPTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("redirects are disallowed for JWKS requests")
		},
	}

	return &JWKSVerifier{
		cfg:    cfg,
		client: client,
		keys:   make(map[string]crypto.PublicKey),
	}, nil
}

func validateEndpointURL(raw string, allowInsecure bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(allowInsecure && u.Scheme == "http") {
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("missing host")
	}
	if !allowInsecure {
		if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			return errors.New("private or loopback IP disallowed")
		}
		if strings.EqualFold(host, "localhost") {
			return errors.New("localhost disallowed")
		}
	}
	return nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Iss string   `json:"iss"`
	Sub string   `json:"sub"`
	Aud audience `json:"aud"`
	Exp int64    `json:"exp"`
	Nbf int64    `json:"nbf"`
}

type audience []string

func (a *audience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(data, &multi); err != nil {
		return err
	}
	*a = multi
	return nil
}

// Verify validates the JWT signature, algorithm, issuer, audience, exp, and nbf.
func (v *JWKSVerifier) Verify(ctx context.Context, rawToken string) (Actor, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return Actor{}, errors.New("malformed jwt")
	}

	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Actor{}, errors.New("invalid jwt header encoding")
	}
	var hdr jwtHeader
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return Actor{}, errors.New("invalid jwt header")
	}

	// Explicit asymmetric algorithm allowlist.
	switch hdr.Alg {
	case "RS256", "ES256", "EdDSA":
	default:
		return Actor{}, fmt.Errorf("unsupported jwt alg %q", hdr.Alg)
	}
	if hdr.Kid == "" {
		return Actor{}, errors.New("missing kid")
	}

	pubKey, err := v.getKey(ctx, hdr.Kid)
	if err != nil {
		return Actor{}, err
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Actor{}, errors.New("invalid jwt signature encoding")
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(hdr.Alg, pubKey, signingInput, sig); err != nil {
		return Actor{}, err
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Actor{}, errors.New("invalid jwt claims encoding")
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return Actor{}, errors.New("invalid jwt claims")
	}

	if claims.Iss != v.cfg.Issuer {
		return Actor{}, errors.New("invalid issuer")
	}
	if claims.Sub == "" {
		return Actor{}, errors.New("missing subject")
	}
	if !contains(claims.Aud, v.cfg.Audience) {
		return Actor{}, errors.New("invalid audience")
	}

	now := time.Now().Unix()
	if claims.Exp == 0 || now >= claims.Exp {
		return Actor{}, errors.New("token expired")
	}
	if claims.Nbf != 0 && now < claims.Nbf {
		return Actor{}, errors.New("token not yet valid")
	}

	return Actor{
		Subject:  claims.Sub,
		Issuer:   claims.Iss,
		Audience: claims.Aud,
		Expires:  time.Unix(claims.Exp, 0),
	}, nil
}

func (v *JWKSVerifier) getKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := time.Since(v.fetchedAt) < v.cfg.CacheTTL
	cooldownActive := !v.lastRefreshAttempt.IsZero() && time.Since(v.lastRefreshAttempt) < v.cfg.RefreshCooldown
	v.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}
	if cooldownActive {
		if ok {
			return key, nil
		}
		return nil, errors.New("unknown kid")
	}

	_, err, _ := v.sfGroup.Do("jwks_refresh", func() (any, error) {
		v.mu.Lock()
		if !v.lastRefreshAttempt.IsZero() && time.Since(v.lastRefreshAttempt) < v.cfg.RefreshCooldown {
			v.mu.Unlock()
			return nil, nil
		}
		v.lastRefreshAttempt = time.Now()
		v.mu.Unlock()

		return nil, v.refreshJWKS(ctx)
	})
	if err != nil {
		return nil, err
	}

	v.mu.RLock()
	key, ok = v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, errors.New("unknown kid")
	}
	return key, nil
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (v *JWKSVerifier) refreshJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc jwksResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("malformed jwks: %w", err)
	}

	parsed := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			continue
		}
		if pub, err := parseJWK(k); err == nil {
			parsed[k.Kid] = pub
		}
	}

	v.mu.Lock()
	v.keys = parsed
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func parseJWK(k jwk) (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, err
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
	case "EC":
		if k.Crv != "P-256" {
			return nil, errors.New("unsupported curve")
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, errors.New("unsupported OKP curve")
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		if len(xBytes) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 public key length")
		}
		return ed25519.PublicKey(xBytes), nil
	default:
		return nil, errors.New("unsupported kty")
	}
}

func verifySignature(alg string, pub crypto.PublicKey, input, sig []byte) error {
	switch alg {
	case "RS256":
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errors.New("key type mismatch for RS256")
		}
		sum := sha256.Sum256(input)
		return rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, sum[:], sig)
	case "ES256":
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("key type mismatch for ES256")
		}
		if len(sig) != 64 {
			return errors.New("invalid ES256 signature length")
		}
		sum := sha256.Sum256(input)
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if !ecdsa.Verify(ecPub, sum[:], r, s) {
			return errors.New("invalid ES256 signature")
		}
		return nil
	case "EdDSA":
		edPub, ok := pub.(ed25519.PublicKey)
		if !ok {
			return errors.New("key type mismatch for EdDSA")
		}
		if !ed25519.Verify(edPub, input, sig) {
			return errors.New("invalid EdDSA signature")
		}
		return nil
	default:
		_ = sha512.New()
		return errors.New("unsupported algorithm")
	}
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
