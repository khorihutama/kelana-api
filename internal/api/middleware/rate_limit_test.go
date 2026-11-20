package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterEmailKeyNormalizesEmailAndKeepsBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(body, remoteAddr string) *gin.Context {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
		context.Request.RemoteAddr = remoteAddr
		return context
	}

	body := `{"email":"  Fulan@Example.COM "}`
	context := newContext(body, "158.140.166.37:1234")
	key := registerEmailKey(context)
	if !strings.HasPrefix(key, "auth-register:email:") {
		t.Fatalf("key %q does not use the email scope", key)
	}
	if len(key) != len("auth-register:email:")+64 {
		t.Fatalf("key %q is not a sha256 hex digest", key)
	}

	restored, err := io.ReadAll(context.Request.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(restored) != body {
		t.Fatalf("body was not restored: %q", string(restored))
	}

	lower := newContext(`{"email":"fulan@example.com"}`, "158.140.166.37:4321")
	if registerEmailKey(lower) != key {
		t.Fatal("email normalization should ignore case, spaces and source port")
	}

	otherIP := newContext(`{"email":"fulan@example.com"}`, "203.0.113.9:1234")
	if registerEmailKey(otherIP) == key {
		t.Fatal("different client IPs must use different buckets")
	}

	if registerEmailKey(newContext(`{}`, "158.140.166.37:1234")) != "" {
		t.Fatal("a missing email must not create an email bucket")
	}
}
