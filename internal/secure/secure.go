// Package secure provides field encryption, keyed hashing and password hashing.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Box encrypts sensitive fields with AES-256-GCM and computes HMAC-SHA256 look-up hashes.
// Ciphertext format: version byte (1) || nonce (12) || sealed.
type Box struct {
	aead    cipher.AEAD
	hmacKey []byte
}

// NewBox derives an encryption key and an HMAC key from a 32+ byte base64 master key.
func NewBox(masterKeyB64 string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("DATA_ENCRYPTION_KEY must be base64: %w", err)
	}
	if len(key) < 32 {
		return nil, errors.New("DATA_ENCRYPTION_KEY must decode to at least 32 bytes")
	}
	encKey := derive(key, "bharatmfg/field-encryption/v1")
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead, hmacKey: derive(key, "bharatmfg/lookup-hmac/v1")}, nil
}

func derive(key []byte, label string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(label))
	return m.Sum(nil)
}

// Encrypt seals plaintext.
func (b *Box) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{1}, nonce...)
	return b.aead.Seal(out, nonce, []byte(plaintext), []byte{1}), nil
}

// Decrypt opens ciphertext produced by Encrypt.
func (b *Box) Decrypt(ct []byte) (string, error) {
	ns := b.aead.NonceSize()
	if len(ct) < 1+ns || ct[0] != 1 {
		return "", errors.New("invalid ciphertext")
	}
	pt, err := b.aead.Open(nil, ct[1:1+ns], ct[1+ns:], []byte{1})
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// Hash returns a hex HMAC of a normalised value, for equality look-ups without decryption.
func (b *Box) Hash(value string) string {
	m := hmac.New(sha256.New, b.hmacKey)
	m.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(m.Sum(nil))
}

// HashIP pseudonymises an IP address for abuse tracking.
func (b *Box) HashIP(ip string) string {
	return b.Hash("ip:" + ip)[:32]
}

// SHA256 returns the raw SHA-256 digest (used for session and API key hashes).
func SHA256(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// RandomToken returns a URL-safe random token with n bytes of entropy.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Mask hides most of a personal value for display to reviewers: r•••@k•••.com, +91•••••3210.
func Mask(channel, v string) string {
	if v == "" {
		return ""
	}
	if channel == "email" {
		at := strings.LastIndex(v, "@")
		if at <= 0 {
			return "•••"
		}
		domain := v[at+1:]
		tld := ""
		if dot := strings.LastIndex(domain, "."); dot > 0 {
			tld = domain[dot:]
		}
		return v[:1] + "•••@" + domain[:1] + "•••" + tld
	}
	if len(v) <= 4 {
		return "••••"
	}
	return v[:min(3, len(v)-4)] + strings.Repeat("•", 5) + v[len(v)-4:]
}

// Argon2id parameters (OWASP-recommended baseline).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
)

// HashPassword returns an encoded Argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against an encoded Argon2id hash in constant time.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var mem uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, mem, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyHash is verified against when a login email is unknown, to equalise timing.
var DummyHash = func() string { h, _ := HashPassword("timing-equaliser-not-a-real-password"); return h }()
