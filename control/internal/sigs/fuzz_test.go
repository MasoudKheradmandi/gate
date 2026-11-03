package sigs

import "testing"

// Arbitrary bundles and signatures must never verify (only the project key can sign)
// and must never crash the verifier.
func FuzzVerify(f *testing.F) {
	f.Add([]byte(`{"serial":1}`), []byte("AAAA"))
	f.Add([]byte{}, []byte{})
	f.Fuzz(func(t *testing.T, data, sig []byte) {
		if Verify(data, sig) == nil {
			t.Fatalf("forged bundle accepted: data=%q sig=%q", data, sig)
		}
	})
}
