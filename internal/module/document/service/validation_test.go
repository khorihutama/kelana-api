package service

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/khorihutama/kelana-api/internal/common/errs"
)

func pdfPayload(size int) []byte {
	data := make([]byte, size)
	copy(data, "%PDF-1.7\n")
	copy(data[size-5:], "%%EOF")
	return data
}

func TestValidatePayload(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}

	tests := []struct {
		name       string
		data       []byte
		wantMIME   string
		wantStatus int
		wantCode   string
	}{
		{name: "pdf", data: pdfPayload(4096), wantMIME: "application/pdf"},
		{name: "jpeg", data: jpeg, wantMIME: "image/jpeg"},
		{name: "png", data: png, wantMIME: "image/png"},
		{name: "plain text", data: []byte("hello, this is not a document"), wantStatus: http.StatusUnsupportedMediaType, wantCode: errs.CodeUnsupportedMedia},
		{name: "empty", data: nil, wantStatus: http.StatusUnsupportedMediaType, wantCode: errs.CodeUnsupportedMedia},
		{name: "over 3 mb", data: bytes.Repeat([]byte{'x'}, int(maxDocumentBytes)+1), wantStatus: http.StatusRequestEntityTooLarge, wantCode: errs.CodeFileTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mimeType, appErr := validatePayload(test.data)
			if test.wantCode != "" {
				if appErr == nil || appErr.Code != test.wantCode || appErr.Status != test.wantStatus {
					t.Fatalf("validatePayload = (%q, %v), want code %s status %d", mimeType, appErr, test.wantCode, test.wantStatus)
				}
				return
			}
			if appErr != nil {
				t.Fatalf("validatePayload: %v", appErr)
			}
			if mimeType != test.wantMIME {
				t.Fatalf("mime = %q, want %q", mimeType, test.wantMIME)
			}
		})
	}
}

func TestExtensionFor(t *testing.T) {
	cases := map[string]string{
		"application/pdf": ".pdf",
		"image/jpeg":      ".jpg",
		"image/png":       ".png",
		"text/plain":      ".bin",
	}
	for mimeType, want := range cases {
		if got := extensionFor(mimeType); got != want {
			t.Fatalf("extensionFor(%q) = %q, want %q", mimeType, got, want)
		}
	}
}
