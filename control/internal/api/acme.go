package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"corvus/control/internal/acme"
	"corvus/control/internal/seal"
	"corvus/control/internal/store"
	"corvus/control/internal/validate"
)

// ACMEService starts background issuance/renewal of an ACME certificate.
type ACMEService interface{ Start(id int64) }

func emailOK(e string) bool {
	at := strings.LastIndex(e, "@")
	return at > 0 && at < len(e)-3 && strings.Contains(e[at:], ".") && len(e) <= 254 && !strings.ContainsAny(e, " \t\r\n")
}

// certRequest is an ACME order as entered in the UI (NPM-compatible fields).
type certRequest struct {
	Email          string          `json:"email"`
	DNSChallenge   bool            `json:"dns_challenge"`
	Challenge      string          `json:"challenge"` // alternative to dns_challenge: http-01 | dns-01
	DNSProvider    string          `json:"dns_provider"`
	DNSCredentials json.RawMessage `json:"dns_credentials"` // certbot ini text, or {"api_token": "..."}
	CredentialsINI string          `json:"dns_credentials_ini"`
	Staging        bool            `json:"staging"`
}

// buildACME validates an order for domains and returns the stored ACME info
// (credentials sealed with the data key). field prefixes error field names.
func (s *Server) buildACME(domains []string, in certRequest, field string) (store.ACMEInfo, []string, validate.Errors) {
	var errs validate.Errors
	add := func(f, code, msg string) {
		errs = append(errs, validate.FieldError{Field: field + f, Code: code, Message: msg})
	}
	var out []string
	wildcard := false
	seen := map[string]bool{}
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		if !validate.Domain(d) || !strings.Contains(d, ".") {
			add("domains", "invalid_domain", fmt.Sprintf("invalid public domain name %q", d))
		}
		wildcard = wildcard || strings.HasPrefix(d, "*.")
		out = append(out, d)
	}
	if len(out) == 0 || len(out) > 50 {
		add("domains", "required", "between 1 and 50 domains are required")
	}
	email := strings.TrimSpace(in.Email)
	if !emailOK(email) {
		add("email", "invalid", "a valid contact email is required")
	}
	challenge := in.Challenge
	if challenge == "" {
		challenge = "http-01"
		if in.DNSChallenge {
			challenge = "dns-01"
		}
	}
	info := store.ACMEInfo{Email: email, Challenge: challenge, Staging: in.Staging}
	switch challenge {
	case "http-01":
		if wildcard {
			add("dns_challenge", "wildcard_needs_dns", "wildcard certificates require the DNS challenge")
		}
	case "dns-01":
		provider := in.DNSProvider
		if provider == "" {
			provider = "cloudflare"
		}
		text := in.CredentialsINI
		if len(in.DNSCredentials) > 0 {
			var str string
			var obj map[string]string
			if json.Unmarshal(in.DNSCredentials, &str) == nil {
				text = str
			} else if json.Unmarshal(in.DNSCredentials, &obj) == nil && obj["api_token"] != "" {
				text = "dns_cloudflare_api_token=" + obj["api_token"]
			}
		}
		cred, err := acme.ParseCredentials(provider, text)
		if err != nil {
			add("dns_credentials", "invalid", err.Error())
			break
		}
		raw, _ := json.Marshal(cred)
		sealed, err := seal.Seal(s.DataKey, string(raw))
		if err != nil {
			add("dns_credentials", "internal", "could not store credentials")
			break
		}
		info.DNSProvider, info.Credentials = provider, sealed
	default:
		add("challenge", "invalid", "challenge must be http-01 or dns-01")
	}
	return info, out, errs
}

func (s *Server) dnsProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, acme.Providers)
}

func (s *Server) issueACME(w http.ResponseWriter, r *http.Request) {
	if s.ACME == nil {
		writeErr(w, http.StatusNotImplemented, "unavailable", "ACME is not available")
		return
	}
	var in struct {
		certRequest
		Domains []string `json:"domains"`
		Name    string   `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	info, domains, errs := s.buildACME(in.Domains, in.certRequest, "")
	if len(errs) > 0 {
		fail(w, errs)
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = domains[0]
	}
	c := &store.Certificate{Name: name, Kind: "acme", Domains: domains, NotAfter: time.Now().UTC(), Status: "pending", AutoRenew: true, ACME: info}
	if err := s.Store.CreateCertificate(r.Context(), c); err != nil {
		fail(w, err)
		return
	}
	c.Challenge = info.Challenge
	s.ACME.Start(c.ID)
	s.audit(r, "certificate.acme_requested", fmt.Sprintf("certificate:%d", c.ID), map[string]any{"domains": domains, "challenge": info.Challenge, "staging": in.Staging})
	writeJSON(w, http.StatusAccepted, c)
}

func (s *Server) renewACME(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := s.Store.CertificateByID(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	if c.Kind != "acme" || s.ACME == nil {
		writeErr(w, http.StatusConflict, "not_acme", "only ACME certificates can be renewed")
		return
	}
	if c.Status != "valid" {
		_ = s.Store.SetCertificateStatus(r.Context(), id, "pending", "")
		c.Status = "pending"
	}
	s.ACME.Start(id)
	s.audit(r, "certificate.acme_renew", fmt.Sprintf("certificate:%d", id), nil)
	writeJSON(w, http.StatusAccepted, c)
}
