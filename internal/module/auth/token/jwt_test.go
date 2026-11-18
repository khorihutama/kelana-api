package token

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateAndParse(t *testing.T) {
	manager := New("01234567890123456789012345678901", 15*time.Minute)
	userID := uuid.Must(uuid.NewV7())
	sessionID := uuid.Must(uuid.NewV7())

	raw, expiresIn, err := manager.Generate(userID, "CUSTOMER", sessionID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if raw == "" || expiresIn != 900 {
		t.Fatalf("unexpected access token result: expiresIn=%d", expiresIn)
	}

	claims, err := manager.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Subject != userID.String() || claims.Role != "CUSTOMER" || claims.SessionID != sessionID.String() {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseRejectsWrongSecretAndExpiredToken(t *testing.T) {
	issuer := New("01234567890123456789012345678901", 15*time.Minute)
	other := New("abcdefghijklmnopqrstuvwxyz123456", 15*time.Minute)
	raw, _, err := issuer.Generate(uuid.Must(uuid.NewV7()), "CUSTOMER", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Parse(raw); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}

	expired := New("01234567890123456789012345678901", -time.Second)
	rawExpired, _, err := expired.Generate(uuid.Must(uuid.NewV7()), "CUSTOMER", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(rawExpired); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
