package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
)

const signatureContext = "hsi-go document url signing v1"

type Signer struct {
	key []byte
	ttl time.Duration
}

func NewSigner(encKey []byte, ttl time.Duration) *Signer {
	mac := hmac.New(sha256.New, encKey)
	mac.Write([]byte(signatureContext))
	return &Signer{key: mac.Sum(nil), ttl: ttl}
}

func (s *Signer) Sign(documentID uuid.UUID, now time.Time) (string, int64) {
	exp := now.Add(s.ttl).Unix()
	return s.signature(documentID, exp), exp
}

func (s *Signer) Verify(documentID uuid.UUID, exp int64, signature string, now time.Time) *errs.AppError {
	expected := s.signature(documentID, exp)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errs.New(errs.CodeSignatureInvalid, 403, "Tanda tangan URL tidak valid")
	}
	if now.Unix() >= exp {
		return errs.New(errs.CodeURLExpired, 410, "URL sudah kedaluwarsa")
	}
	return nil
}

func (s *Signer) signature(documentID uuid.UUID, exp int64) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(documentID.String()))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
