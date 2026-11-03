package auth

import (
	"testing"
	"time"
)

func TestAPR1KnownVector(t *testing.T) {
	// Vector produced by `openssl passwd -apr1 -salt xxxxxxxx password`.
	if got := apr1("password", "xxxxxxxx"); got != "$apr1$xxxxxxxx$dxHfLAsjHkDRmG83UXe8K0" {
		t.Fatalf("apr1 = %s", got)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if b, _ := l.Blocked("ip"); b {
			t.Fatal("blocked too early")
		}
		l.Fail("ip")
	}
	if b, _ := l.Blocked("ip"); !b {
		t.Fatal("not blocked after max failures")
	}
	l.Reset("ip")
	if b, _ := l.Blocked("ip"); b {
		t.Fatal("still blocked after reset")
	}
}

func TestSetupTokenSingleUse(t *testing.T) {
	s := NewSetupToken()
	v := s.Value()
	if s.Consume("wrong") || !s.Consume(v) || s.Consume(v) {
		t.Fatal("setup token must be single use and exact")
	}
}
