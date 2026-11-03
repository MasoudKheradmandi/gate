package acme

import (
	"errors"
	"strings"
)

// Provider describes a supported DNS-01 provider and the credential template
// shown in the UI (certbot ini format, compatible with Nginx Proxy Manager).
type Provider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Template string `json:"credentials_template"`
}

// PlaceholderToken is the example value in the template; it is rejected so a
// forgotten edit fails at save time instead of during issuance.
const PlaceholderToken = "0123456789abcdef0123456789abcdef01234567"

var Providers = []Provider{
	{ID: "cloudflare", Name: "Cloudflare", Template: "# Cloudflare API token\ndns_cloudflare_api_token=" + PlaceholderToken},
}

// ParseCredentials reads certbot-style "key = value" credentials for a provider.
// Cloudflare accepts dns_cloudflare_api_token (recommended) or the legacy
// dns_cloudflare_email + dns_cloudflare_api_key pair.
func ParseCredentials(provider, text string) (Credentials, error) {
	kv := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return Credentials{}, errors.New("each line must be key = value")
		}
		kv[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	switch provider {
	case "cloudflare":
		c := Credentials{APIToken: kv["dns_cloudflare_api_token"], Email: kv["dns_cloudflare_email"], APIKey: kv["dns_cloudflare_api_key"]}
		if c.APIToken == PlaceholderToken {
			return c, errors.New("replace the example token with your Cloudflare API token")
		}
		if c.APIToken == "" && (c.Email == "" || c.APIKey == "") {
			return c, errors.New("dns_cloudflare_api_token is required (or dns_cloudflare_email + dns_cloudflare_api_key)")
		}
		return c, nil
	}
	return Credentials{}, errors.New("unsupported DNS provider")
}
