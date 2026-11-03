package seal

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	k := bytes.Repeat([]byte{7}, 32)
	s, err := Seal(k, "cf-token")
	if err != nil || s == "cf-token" {
		t.Fatal(err)
	}
	if p, err := Open(k, s); err != nil || p != "cf-token" {
		t.Fatalf("open: %v %q", err, p)
	}
	if _, err := Open(bytes.Repeat([]byte{8}, 32), s); err == nil {
		t.Fatal("wrong key accepted")
	}
	if v, _ := Seal(k, ""); v != "" {
		t.Fatal("empty should stay empty")
	}
}
