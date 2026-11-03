package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret (base32, RFC 4226 recommendation).
func NewTOTPSecret() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b32.EncodeToString(b)
}

// TOTPURL is the otpauth:// URI for authenticator apps.
func TOTPURL(issuer, account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func hotp(secret []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	m := hmac.New(sha1.New, secret)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1_000_000)
}

// TOTPStep returns the 30-second time step that code matches (±1 step for clock
// skew), or -1. Callers must reject steps not greater than the last accepted one
// to prevent replay.
func TOTPStep(secret, code string, now time.Time) int64 {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(code) != 6 {
		return -1
	}
	step := now.Unix() / 30
	for _, d := range []int64{0, -1, 1} {
		if subtle.ConstantTimeCompare([]byte(hotp(key, uint64(step+d))), []byte(code)) == 1 {
			return step + d
		}
	}
	return -1
}

// NewRecoveryCodes returns n human-friendly one-time codes and their hashes.
func NewRecoveryCodes(n int) (codes, hashes []string) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	for i := 0; i < n; i++ {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		var sb strings.Builder
		for j, c := range b {
			if j == 5 {
				sb.WriteByte('-')
			}
			sb.WriteByte(alphabet[int(c)%len(alphabet)])
		}
		codes = append(codes, sb.String())
		hashes = append(hashes, RecoveryHash(sb.String()))
	}
	return
}

func RecoveryHash(code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

// MFAChallenges holds short-lived "password verified, second factor pending" tokens.
type MFAChallenges struct {
	mu      sync.Mutex
	pending map[string]mfaPending
}

type mfaPending struct {
	userID   int64
	expires  time.Time
	attempts int
}

func NewMFAChallenges() *MFAChallenges {
	return &MFAChallenges{pending: map[string]mfaPending{}}
}

func (m *MFAChallenges) Start(userID int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, p := range m.pending {
		if now.After(p.expires) {
			delete(m.pending, k)
		}
	}
	tok := RandomToken(24)
	m.pending[tok] = mfaPending{userID: userID, expires: now.Add(5 * time.Minute)}
	return tok
}

// Peek returns the user of a live challenge and counts an attempt (max 5).
func (m *MFAChallenges) Peek(tok string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[tok]
	if !ok || time.Now().After(p.expires) || p.attempts >= 5 {
		delete(m.pending, tok)
		return 0, false
	}
	p.attempts++
	m.pending[tok] = p
	return p.userID, true
}

func (m *MFAChallenges) Finish(tok string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, tok)
}
