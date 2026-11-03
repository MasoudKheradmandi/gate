package acme

import "testing"

func TestParseCredentials(t *testing.T) {
	c, err := ParseCredentials("cloudflare", "# Cloudflare API token\ndns_cloudflare_api_token = abc123\n")
	if err != nil || c.APIToken != "abc123" {
		t.Fatalf("token: %+v %v", c, err)
	}
	if _, err := ParseCredentials("cloudflare", Providers[0].Template); err == nil {
		t.Fatal("unchanged template placeholder must be rejected")
	}
	if c, err := ParseCredentials("cloudflare", "dns_cloudflare_email=a@b.c\ndns_cloudflare_api_key=k"); err != nil || c.APIKey != "k" {
		t.Fatalf("legacy key: %v", err)
	}
	if _, err := ParseCredentials("cloudflare", "# empty"); err == nil {
		t.Fatal("missing token accepted")
	}
}

func TestDirectoryScope(t *testing.T) {
	s := &Service{Directory: "https://pebble:14000/dir", DirectorySuffix: ".corvus.test"}
	if got := s.directory(false, []string{"a.corvus.test", "*.b.corvus.test"}); got != s.Directory {
		t.Fatalf("test domains: %s", got)
	}
	if got := s.directory(false, []string{"www.example.com"}); got == s.Directory {
		t.Fatal("real domain sent to the test CA")
	}
	if got := s.directory(false, []string{"a.corvus.test", "www.example.com"}); got == s.Directory {
		t.Fatal("mixed domains sent to the test CA")
	}
	if !sameCA("https://pebble:14000/my-account/1", "https://pebble:14000/dir") || sameCA("https://pebble:14000/my-account/1", "https://acme-v02.api.letsencrypt.org/directory") {
		t.Fatal("sameCA")
	}
}
