package auth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/semaian/platform/services/api/internal/platform/auth"
)

func signEdDSA(t *testing.T, priv ed25519.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	hdr, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": kid, "typ": "JWT"})
	pl, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(pl)
	sig := ed25519.Sign(priv, []byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestJWKSSingleflightCooldownAndEd25519(t *testing.T) {
	pubEd, privEd, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519 keygen failed: %v", err)
	}

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []any{
				map[string]any{
					"kty": "OKP", "crv": "Ed25519", "kid": "ed-1", "use": "sig", "alg": "EdDSA",
					"x": base64.RawURLEncoding.EncodeToString(pubEd),
				},
				map[string]any{
					"kty": "OKP", "crv": "Ed25519", "kid": "ed-short", "use": "sig", "alg": "EdDSA",
					"x": base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}),
				},
				map[string]any{
					"kty": "OKP", "crv": "Ed25519", "kid": "ed-long", "use": "sig", "alg": "EdDSA",
					"x": base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
				},
				map[string]any{
					"kty": "OKP", "crv": "Ed25519", "kid": "ed-badb64", "use": "sig", "alg": "EdDSA",
					"x": "!!!not-base64!!!",
				},
			},
		})
	}))
	defer srv.Close()

	verifier, err := auth.NewJWKSVerifier(auth.JWKSVerifierConfig{
		Issuer:          "https://issuer.example",
		Audience:        "semaian-api",
		JWKSURL:         srv.URL,
		RefreshCooldown: 100 * time.Millisecond,
		AllowInsecure:   true,
	})
	if err != nil {
		t.Fatalf("NewJWKSVerifier failed: %v", err)
	}

	now := time.Now().Unix()

	t.Run("concurrent unknown kid triggers only one refresh", func(t *testing.T) {
		fetchCount.Store(0)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tok := signEdDSA(t, privEd, "unknown-kid", map[string]any{
					"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
				})
				_, _ = verifier.Verify(context.Background(), tok)
			}()
		}
		wg.Wait()
		if got := fetchCount.Load(); got != 1 {
			t.Fatalf("expected 1 JWKS fetch for concurrent unknown kids, got %d", got)
		}
	})

	t.Run("cooldown blocks immediate repeat unknown kid refresh", func(t *testing.T) {
		tok := signEdDSA(t, privEd, "unknown-kid-2", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-1", "exp": now + 300,
		})
		_, _ = verifier.Verify(context.Background(), tok)
		if got := fetchCount.Load(); got != 1 {
			t.Fatalf("expected cooldown to prevent second fetch, got %d", got)
		}
	})

	t.Run("valid Ed25519 token succeeds", func(t *testing.T) {
		tok := signEdDSA(t, privEd, "ed-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-ed", "exp": now + 300,
		})
		actor, err := verifier.Verify(context.Background(), tok)
		if err != nil || actor.Subject != "sub-ed" {
			t.Fatalf("expected valid EdDSA actor, got err=%v", err)
		}
	})

	t.Run("short, oversized, and malformed Ed25519 keys are rejected", func(t *testing.T) {
		for _, kid := range []string{"ed-short", "ed-long", "ed-badb64"} {
			tok := signEdDSA(t, privEd, kid, map[string]any{
				"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-ed", "exp": now + 300,
			})
			if _, err := verifier.Verify(context.Background(), tok); err == nil {
				t.Fatalf("expected error for invalid Ed25519 key %q", kid)
			}
		}
	})

	t.Run("cooldown expiration allows future refresh", func(t *testing.T) {
		time.Sleep(110 * time.Millisecond)
		before := fetchCount.Load()
		tok := signEdDSA(t, privEd, "ed-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-ed", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tok); err != nil {
			t.Fatalf("expected valid token after cooldown, got %v", err)
		}
		_ = before
	})

	t.Run("context cancellation does not permanently block future refresh", func(t *testing.T) {
		time.Sleep(110 * time.Millisecond)
		cctx, cancel := context.WithCancel(context.Background())
		cancel()
		tokUnknown := signEdDSA(t, privEd, "unknown-after-cancel", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-ed", "exp": now + 300,
		})
		_, _ = verifier.Verify(cctx, tokUnknown)

		time.Sleep(110 * time.Millisecond)
		tokValid := signEdDSA(t, privEd, "ed-1", map[string]any{
			"iss": "https://issuer.example", "aud": "semaian-api", "sub": "sub-ed", "exp": now + 300,
		})
		if _, err := verifier.Verify(context.Background(), tokValid); err != nil {
			t.Fatalf("expected valid token to succeed after canceled refresh, got %v", err)
		}
	})
}
