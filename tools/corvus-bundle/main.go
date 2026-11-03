// corvus-bundle signs managed rule bundles (body signatures + virtual-patch templates).
//
//	corvus-bundle keygen <dir>              writes bundle-signing.key (private, keep offline) and .pub
//	corvus-bundle sign <bundle.json> <key>  writes <bundle.json>.sig (base64 Ed25519 signature)
//	corvus-bundle verify <bundle.json> <pub-base64>
//
// The public key is compiled into corvus-control (control/internal/sigs/key.go).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: corvus-bundle keygen <dir> | sign <bundle.json> <key> | verify <bundle.json> <pub>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "keygen":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		check(err)
		dir := os.Args[2]
		check(os.MkdirAll(dir, 0o700))
		keyPath := filepath.Join(dir, "bundle-signing.key")
		if _, err := os.Stat(keyPath); err == nil {
			fail("refusing to overwrite " + keyPath)
		}
		check(os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(priv.Seed())+"\n"), 0o600))
		check(os.WriteFile(filepath.Join(dir, "bundle-signing.pub"), []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644))
		fmt.Println("public key:", base64.StdEncoding.EncodeToString(pub))
	case "sign":
		if len(os.Args) != 4 {
			fail("usage: sign <bundle.json> <key>")
		}
		data, err := os.ReadFile(os.Args[2])
		check(err)
		raw, err := os.ReadFile(os.Args[3])
		check(err)
		seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		check(err)
		if len(seed) != ed25519.SeedSize {
			fail("invalid private key")
		}
		sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
		check(os.WriteFile(os.Args[2]+".sig", []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644))
		fmt.Println("signed", os.Args[2])
	case "verify":
		if len(os.Args) != 4 {
			fail("usage: verify <bundle.json> <pub-base64>")
		}
		data, err := os.ReadFile(os.Args[2])
		check(err)
		sigRaw, err := os.ReadFile(os.Args[2] + ".sig")
		check(err)
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigRaw)))
		check(err)
		pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Args[3]))
		check(err)
		if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, data, sig) {
			fail("signature INVALID")
		}
		fmt.Println("signature OK")
	default:
		fail("unknown command " + os.Args[1])
	}
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
