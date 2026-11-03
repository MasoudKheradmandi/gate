package auth

import (
	"testing"
	"time"
)

func TestTOTPRFC6238(t *testing.T) {
	// RFC 6238 appendix B, SHA1 secret "12345678901234567890" (6-digit truncation of the published 8-digit values).
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"}
	for ts, code := range cases {
		if step := TOTPStep(secret, code, time.Unix(ts, 0)); step != ts/30 {
			t.Errorf("t=%d code %s: step %d", ts, code, step)
		}
	}
	if TOTPStep(secret, "000000", time.Unix(59, 0)) != -1 {
		t.Error("wrong code accepted")
	}
	// ±1 step tolerance
	if TOTPStep(secret, "287082", time.Unix(59+30, 0)) == -1 {
		t.Error("previous step not accepted")
	}
	if TOTPStep(secret, "287082", time.Unix(59+90, 0)) != -1 {
		t.Error("code 3 steps old accepted")
	}
}

func TestMFAAttempts(t *testing.T) {
	m := NewMFAChallenges()
	tok := m.Start(7)
	for i := 0; i < 5; i++ {
		if uid, ok := m.Peek(tok); !ok || uid != 7 {
			t.Fatalf("attempt %d rejected", i)
		}
	}
	if _, ok := m.Peek(tok); ok {
		t.Fatal("6th attempt allowed")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes := NewRecoveryCodes(10)
	if len(codes) != 10 || len(hashes) != 10 || RecoveryHash(" "+codes[3]+" ") != hashes[3] {
		t.Fatal("recovery codes")
	}
}
