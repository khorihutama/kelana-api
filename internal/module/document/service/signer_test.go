package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
)

func TestSignerValidSignature(t *testing.T) {
	signer := NewSigner([]byte("0123456789abcdef0123456789abcdef"), 5*time.Minute)
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	documentID := uuid.Must(uuid.NewV7())

	signature, exp := signer.Sign(documentID, now)
	if exp != now.Add(5*time.Minute).Unix() {
		t.Fatalf("exp = %d", exp)
	}
	if appErr := signer.Verify(documentID, exp, signature, now.Add(4*time.Minute)); appErr != nil {
		t.Fatalf("Verify: %v", appErr)
	}
}

func TestSignerRejectsExpiredURL(t *testing.T) {
	signer := NewSigner([]byte("0123456789abcdef0123456789abcdef"), 5*time.Minute)
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	documentID := uuid.Must(uuid.NewV7())
	signature, exp := signer.Sign(documentID, now)

	appErr := signer.Verify(documentID, exp, signature, now.Add(5*time.Minute))
	if appErr == nil || appErr.Code != errs.CodeURLExpired {
		t.Fatalf("Verify expired = %v, want URL_EXPIRED", appErr)
	}
}

func TestSignerRejectsTampering(t *testing.T) {
	signer := NewSigner([]byte("0123456789abcdef0123456789abcdef"), 5*time.Minute)
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	documentID := uuid.Must(uuid.NewV7())
	signature, exp := signer.Sign(documentID, now)

	if appErr := signer.Verify(uuid.Must(uuid.NewV7()), exp, signature, now); appErr == nil || appErr.Code != errs.CodeSignatureInvalid {
		t.Fatalf("Verify other document = %v, want SIGNATURE_INVALID", appErr)
	}
	if appErr := signer.Verify(documentID, exp+1, signature, now); appErr == nil || appErr.Code != errs.CodeSignatureInvalid {
		t.Fatalf("Verify changed exp = %v, want SIGNATURE_INVALID", appErr)
	}
	if appErr := signer.Verify(documentID, exp, signature[:len(signature)-1]+"x", now); appErr == nil || appErr.Code != errs.CodeSignatureInvalid {
		t.Fatalf("Verify changed signature = %v, want SIGNATURE_INVALID", appErr)
	}
}
