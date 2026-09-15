package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testService() *Service {
	return &Service{Secret: []byte("test-secret"), Expiry: time.Hour}
}

func signed(t *testing.T, s *Service, claims jwt.MapClaims, method jwt.SigningMethod, secret []byte) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	out, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return out
}

func TestVerifyValidToken(t *testing.T) {
	s := testService()
	token := signed(t, s, jwt.MapClaims{
		"sub": 42,
		"usr": "trader",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}, jwt.SigningMethodHS256, s.Secret)

	id, name, err := s.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id != 42 {
		t.Errorf("user id = %d, want 42", id)
	}
	if name != "trader" {
		t.Errorf("username = %q, want trader", name)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	s := testService()
	token := signed(t, s, jwt.MapClaims{
		"sub": 1,
		"usr": "x",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}, jwt.SigningMethodHS256, s.Secret)

	if _, _, err := s.Verify(token); err == nil {
		t.Fatal("expected an expired token to be rejected")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	s := testService()
	token := signed(t, s, jwt.MapClaims{
		"sub": 1, "usr": "x", "exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodHS256, []byte("a-different-secret"))

	if _, _, err := s.Verify(token); err == nil {
		t.Fatal("expected a token signed with another secret to be rejected")
	}
}

// Accepting "alg: none" would let anyone mint a token for any user, so
// the signing method must be pinned to HMAC.
func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	s := testService()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": 1, "usr": "attacker", "exp": time.Now().Add(time.Hour).Unix(),
	})
	token, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, _, err := s.Verify(token); err == nil {
		t.Fatal("an unsigned token must be rejected")
	}
}

func TestVerifyMalformedTokens(t *testing.T) {
	s := testService()
	for _, token := range []string{"", "not-a-jwt", "a.b.c", "....."} {
		if _, _, err := s.Verify(token); err == nil {
			t.Errorf("Verify(%q) should have failed", token)
		}
	}
}
