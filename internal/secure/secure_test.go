package secure

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	b, err := NewBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEncryptRoundTrip(t *testing.T) {
	b := testBox(t)
	ct, err := b.Encrypt("owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ct), "owner") {
		t.Fatal("plaintext visible in ciphertext")
	}
	pt, err := b.Decrypt(ct)
	if err != nil || pt != "owner@example.com" {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
	ct[len(ct)-1] ^= 1
	if _, err := b.Decrypt(ct); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if b.Hash("A@B.com ") != b.Hash("a@b.com") {
		t.Fatal("hash should normalise case/whitespace")
	}
}

func TestShortKeyRejected(t *testing.T) {
	if _, err := NewBox(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse battery staple", h) {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword("wrong", h) || VerifyPassword("x", "garbage") {
		t.Fatal("invalid password accepted")
	}
}

func TestMask(t *testing.T) {
	if got := Mask("email", "rakesh@kalin.example"); got != "r•••@k•••.example" {
		t.Errorf("Mask email = %q", got)
	}
	if got := Mask("phone", "+919876543210"); got != "+91•••••3210" {
		t.Errorf("Mask phone = %q", got)
	}
}
