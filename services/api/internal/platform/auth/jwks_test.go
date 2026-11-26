package auth_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/semaian/platform/services/api/internal/platform/auth"
)

func signRS256(t *testing.T, priv *rsa.PrivateKey, alg, kid string, claims map[string]any) string {
	t.Helper()
	hdr, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	pl, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(pl)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func rsaJWK(kid string, pub *rsa.PublicKey) map[string]any {
	return map[string]any{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func TestJWKSVerifier(t *testing.T) {
	priv1, _ := rsa.GenerateKey(rand.Reader, 2048)
	priv2, _ := rsa.GenerateKey(rand.Reader, 2048)

	var activeKid atomic.Value
	activeKid.Store("kid-1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/jwks.json", http.StatusFound)
			return
		}
		if r.URL.Path == "/malformed" {
			_, _ = w.Write([]byte("{not-json"))
			return
		}
		kid := activeKid.Load().(string)
		pub := &priv1.PublicKey
		if kid == "kid-2" {
			pub = &priv2.PublicKey
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []any{rsaJWK(kid, pub)},
		})
	}))
	defer srv.Close()

	verifier, err := auth.NewJWKSVerifier(auth.JWKSVerifierConfig{
		Issuer:          "https://issuer.example",
		Audience:        "semaian-api",
		JWKSURL:         srv.URL + "/jwks.json",
		RefreshCooldown: 10 * time.Millisecond,
		AllowInsecure:   true,
	})
	if err != nil {
		t.Fatalf("NewJWKSVerifier failed: %v", err)
	}

	now := time.Now().Unix()

	t.Run("valid token", func(t *testing.T) {
		tok := signRS256(t, priv1, "RS256", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		actor, err := verifier.Verify(context.Background(), tok)
		if err != nil || actor.Subject != "sub-1" {
			t.Fatalf("expected valid actor, got err=%v actor=%+v", err, actor)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		tok := signRS256(t, priv2, "RS256", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for invalid signature")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		tok := signRS256(t, priv1, "RS256", "kid-1", map[string]any{
			"iss": "https://evil.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for wrong issuer")
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		tok := signRS256(t, priv1, "RS256", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "other-api", "sub": "sub-1", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for wrong audience")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		tok := signRS256(t, priv1, "RS256", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now - 10,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for expired token")
		}
	})

	t.Run("not yet valid", func(t *testing.T) {
		tok := signRS256(t, priv1, "RS256", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "nbf": now + 60, "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for nbf in future")
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		tok := signRS256(t, priv1, "none", "kid-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected error for alg=none")
		}
	})

	t.Run("key rotation on unknown kid", func(t *testing.T) {
		time.Sleep(15 * time.Millisecond)
		activeKid.Store("kid-2")
		tok := signRS256(t, priv2, "RS256", "kid-2", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-2", "exp": now + 300,
		})
		actor, err := verifier.Verify(context.Background(), tok)
		if err != nil || actor.Subject != "sub-2" {
			t.Fatalf("expected key rotation to succeed, got err=%v", err)
		}
	})

	t.Run("SSRF private IP and redirect blocked", func(t *testing.T) {
		if _, err := auth.NewJWKSVerifier(auth.JWKSVerifierConfig{
			Issuer:   "https://issuer.example",
			Audience: "semaian-api",
			JWKSURL:  "https://127.0.0.1/jwks.json",
		}); err == nil {
			t.Fatal("expected SSRF private IP to be rejected")
		}
		redirVerifier, _ := auth.NewJWKSVerifier(auth.JWKSVerifierConfig{
			Issuer:        "https://issuer.example",
			Audience:      "semaian-api",
			JWKSURL:       srv.URL + "/redirect",
			AllowInsecure: true,
		})
		tok := signRS256(t, priv1, "RS256", "kid-99", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		if _, err := redirVerifier.Verify(context.Background(), tok); err == nil {
			t.Fatal("expected redirect to be blocked")
		}
	})
}
