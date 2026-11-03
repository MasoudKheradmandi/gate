package api

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"corvus/control/internal/auth"
	"corvus/control/internal/seal"
	"corvus/control/internal/sso"
	"corvus/control/internal/store"
)

// ------------------------------------------------------------------ mock IdP

type mockIdP struct {
	t        *testing.T
	srv      *httptest.Server
	key      *rsa.PrivateKey
	clientID string
	secret   string

	mu     sync.Mutex
	codes  map[string]mockCode
	claims map[string]any       // identity returned for the next code
	mutate func(map[string]any) // final tweak of the ID token claims
}

type mockCode struct{ nonce, challenge, redirect string }

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func newMockIdP(t *testing.T) *mockIdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	m := &mockIdP{t: t, key: key, clientID: "corvus-console", secret: "s3cret-value", codes: map[string]mockCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		u := m.srv.URL
		writeJSON(w, 200, map[string]any{"issuer": u, "authorization_endpoint": u + "/authorize", "token_endpoint": u + "/token",
			"jwks_uri": u + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, sec, ok := r.BasicAuth()
		if !ok {
			id, sec = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		}
		if id != m.clientID || sec != m.secret {
			writeJSON(w, 401, map[string]string{"error": "invalid_client"})
			return
		}
		m.mu.Lock()
		c, found := m.codes[r.PostForm.Get("code")]
		delete(m.codes, r.PostForm.Get("code"))
		claims := map[string]any{}
		for k, v := range m.claims {
			claims[k] = v
		}
		mutate := m.mutate
		m.mu.Unlock()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !found || b64(sum[:]) != c.challenge || r.PostForm.Get("redirect_uri") != c.redirect {
			writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
			return
		}
		now := time.Now()
		claims["iss"], claims["aud"], claims["nonce"] = m.srv.URL, m.clientID, c.nonce
		claims["iat"], claims["exp"] = now.Unix(), now.Add(5*time.Minute).Unix()
		if mutate != nil {
			mutate(claims)
		}
		writeJSON(w, 200, map[string]any{"access_token": "at-" + auth.RandomToken(8), "token_type": "Bearer", "expires_in": 300, "id_token": m.sign(claims)})
	})
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockIdP) sign(claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	p, _ := json.Marshal(claims)
	in := b64(h) + "." + b64(p)
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.key, crypto.SHA256, sum[:])
	if err != nil {
		m.t.Fatal(err)
	}
	return in + "." + b64(sig)
}

// authorize plays the IdP login page: checks the request and returns the redirect back.
func (m *mockIdP) authorize(t *testing.T, loc string) string {
	t.Helper()
	u, err := url.Parse(loc)
	if err != nil || !strings.HasPrefix(loc, m.srv.URL+"/authorize") {
		t.Fatalf("start did not redirect to the IdP: %q", loc)
	}
	q := u.Query()
	if q.Get("client_id") != m.clientID || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("state") == "" || q.Get("nonce") == "" || !strings.Contains(q.Get("scope"), "openid") {
		t.Fatalf("bad authorization request: %v", q)
	}
	code := auth.RandomToken(12)
	m.mu.Lock()
	m.codes[code] = mockCode{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri")}
	m.mu.Unlock()
	return q.Get("redirect_uri") + "?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
}

// ------------------------------------------------------------------ console

type ssoEnv struct {
	t       *testing.T
	s       *Server
	console *httptest.Server
	idp     *mockIdP
}

func newSSOEnv(t *testing.T, mutate func(*sso.Settings)) *ssoEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "corvus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	// a local administrator exists already (so SSO users can be demoted)
	if _, err := st.CreateUser(context.Background(), "root@example.com", "Root", "x", "admin"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: st, DataKey: key, Limiter: auth.NewLimiter(100, time.Minute), AcctLimiter: auth.NewLimiter(100, time.Minute), MFA: auth.NewMFAChallenges()}
	e := &ssoEnv{t: t, s: s, idp: newMockIdP(t)}
	e.console = httptest.NewServer(s.Handler())
	t.Cleanup(e.console.Close)

	sealed, err := seal.Seal(key, e.idp.secret)
	if err != nil {
		t.Fatal(err)
	}
	cfg := sso.Defaults()
	cfg.Enabled, cfg.Issuer, cfg.ClientID, cfg.ClientSecret = true, e.idp.srv.URL, e.idp.clientID, sealed
	cfg.DisplayName, cfg.AutoProvision = "Sign in with Mock", true
	cfg.RoleClaim, cfg.AdminValues, cfg.OperatorValues = "groups", []string{"corvus-admins"}, []string{"corvus-ops"}
	if mutate != nil {
		mutate(&cfg)
	}
	b, _ := json.Marshal(cfg)
	if err := st.SetMeta(context.Background(), sso.MetaKey, string(b)); err != nil {
		t.Fatal(err)
	}
	e.identity("alice@example.com", "corvus-admins")
	return e
}

func (e *ssoEnv) identity(email string, groups ...string) {
	e.idp.mu.Lock()
	defer e.idp.mu.Unlock()
	e.idp.claims = map[string]any{"sub": "sub-" + email, "email": email, "email_verified": true, "name": "User " + email, "groups": groups}
}

func (e *ssoEnv) browser() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *ssoEnv) get(c *http.Client, u string) *http.Response {
	e.t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// login runs the whole browser flow and returns the final redirect target.
func (e *ssoEnv) login(c *http.Client) string {
	e.t.Helper()
	resp := e.get(c, e.console.URL+"/api/auth/oidc/start")
	if resp.StatusCode != http.StatusFound {
		e.t.Fatalf("start: status %d", resp.StatusCode)
	}
	back := e.idp.authorize(e.t, resp.Header.Get("Location"))
	resp = e.get(c, back)
	if resp.StatusCode != http.StatusFound {
		e.t.Fatalf("callback: status %d", resp.StatusCode)
	}
	return resp.Header.Get("Location")
}

func (e *ssoEnv) me(c *http.Client) *store.User {
	e.t.Helper()
	resp, err := c.Get(e.console.URL + "/api/auth/me")
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var u store.User
	_ = json.NewDecoder(resp.Body).Decode(&u)
	return &u
}

func (e *ssoEnv) audited(action string) int {
	e.t.Helper()
	list, err := e.s.Store.ListAudit(context.Background(), 1000)
	if err != nil {
		e.t.Fatal(err)
	}
	n := 0
	for _, a := range list {
		if a.Action == action {
			n++
		}
	}
	return n
}

func wantErr(t *testing.T, loc, code string) {
	t.Helper()
	if loc != "/login?sso_error="+code {
		t.Fatalf("redirect = %q, want sso_error=%s", loc, code)
	}
}

// ------------------------------------------------------------------ tests

func TestSSOHappyPathProvisionAndRoleSync(t *testing.T) {
	e := newSSOEnv(t, nil)
	c := e.browser()
	if loc := e.login(c); loc != "/" {
		t.Fatalf("redirect = %q, want /", loc)
	}
	u := e.me(c)
	if u == nil || u.Email != "alice@example.com" || u.Role != "admin" || u.Name != "User alice@example.com" {
		t.Fatalf("me = %+v", u)
	}
	if e.audited("auth.sso_login") != 1 {
		t.Fatal("auth.sso_login not audited")
	}
	// the provisioned account has no usable password
	if db, _ := e.s.Store.UserByEmail(context.Background(), "alice@example.com"); auth.CheckPassword(db.PasswordHash(), "") {
		t.Fatal("empty password accepted")
	}

	// next sign-in: IdP now says operator → role follows the mapping, old sessions end
	e.identity("alice@example.com", "corvus-ops", "other")
	c2 := e.browser()
	if loc := e.login(c2); loc != "/" {
		t.Fatalf("redirect = %q", loc)
	}
	if u := e.me(c2); u == nil || u.Role != "operator" {
		t.Fatalf("role not synced: %+v", u)
	}
	if e.me(c) != nil {
		t.Fatal("old session survived a role change")
	}
	// no matching group → default role viewer
	e.identity("alice@example.com", "nobody")
	c3 := e.browser()
	e.login(c3)
	if u := e.me(c3); u == nil || u.Role != "viewer" {
		t.Fatalf("default role: %+v", u)
	}
}

func TestSSOLastAdminIsNotDemoted(t *testing.T) {
	e := newSSOEnv(t, nil)
	ctx := context.Background()
	root, _ := e.s.Store.UserByEmail(ctx, "root@example.com")
	e.identity("root@example.com", "nobody")
	c := e.browser()
	if loc := e.login(c); loc != "/" {
		t.Fatalf("redirect = %q", loc)
	}
	if u := e.me(c); u == nil || u.ID != root.ID || u.Role != "admin" {
		t.Fatalf("last admin demoted: %+v", u)
	}
}

func TestSSOBadState(t *testing.T) {
	e := newSSOEnv(t, nil)
	c := e.browser()
	resp := e.get(c, e.console.URL+"/api/auth/oidc/start")
	back := e.idp.authorize(t, resp.Header.Get("Location"))
	u, _ := url.Parse(back)
	q := u.Query()
	q.Set("state", "forged")
	u.RawQuery = q.Encode()
	wantErr(t, e.get(c, u.String()).Header.Get("Location"), "state")

	// a valid state presented by another browser (no binding cookie) is rejected too
	resp = e.get(c, e.console.URL+"/api/auth/oidc/start")
	back = e.idp.authorize(t, resp.Header.Get("Location"))
	wantErr(t, e.get(e.browser(), back).Header.Get("Location"), "state")
	// and the state is single-use
	wantErr(t, e.get(c, back).Header.Get("Location"), "state")
	if e.audited("auth.sso_failed") != 3 {
		t.Fatalf("auth.sso_failed audited %d times", e.audited("auth.sso_failed"))
	}
}

func TestSSOBadNonce(t *testing.T) {
	e := newSSOEnv(t, nil)
	e.idp.mutate = func(c map[string]any) { c["nonce"] = "something-else" }
	wantErr(t, e.login(e.browser()), "token_invalid")
}

func TestSSOWrongAudience(t *testing.T) {
	e := newSSOEnv(t, nil)
	e.idp.mutate = func(c map[string]any) { c["aud"] = "another-app" }
	wantErr(t, e.login(e.browser()), "token_invalid")
}

func TestSSOWrongIssuerAndExpired(t *testing.T) {
	e := newSSOEnv(t, nil)
	e.idp.mutate = func(c map[string]any) { c["iss"] = "https://evil.example" }
	wantErr(t, e.login(e.browser()), "token_invalid")
	e.idp.mutate = func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }
	wantErr(t, e.login(e.browser()), "token_invalid")
}

func TestSSOBadSignature(t *testing.T) {
	e := newSSOEnv(t, nil)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	good := e.idp.key
	e.idp.key = other // tokens signed with a key not in the JWKS (JWKS still serves the original n/e)
	defer func() { e.idp.key = good }()
	// JWKS handler closes over the original key, so verification must fail
	wantErr(t, e.login(e.browser()), "token_invalid")
}

func TestSSODisallowedDomain(t *testing.T) {
	e := newSSOEnv(t, func(s *sso.Settings) { s.AllowedDomains = []string{"corp.example"} })
	e.identity("mallory@evil.example", "corvus-admins")
	wantErr(t, e.login(e.browser()), "domain")
	if _, err := e.s.Store.UserByEmail(context.Background(), "mallory@evil.example"); err == nil {
		t.Fatal("user created for a disallowed domain")
	}
	e.identity("bob@corp.example", "corvus-ops")
	if loc := e.login(e.browser()); loc != "/" {
		t.Fatalf("allowed domain rejected: %q", loc)
	}
}

func TestSSOUnverifiedEmail(t *testing.T) {
	e := newSSOEnv(t, nil)
	e.idp.mutate = func(c map[string]any) { c["email_verified"] = false }
	wantErr(t, e.login(e.browser()), "email_unverified")
}

func TestSSODisabledUser(t *testing.T) {
	e := newSSOEnv(t, nil)
	ctx := context.Background()
	u, err := e.s.Store.CreateUser(ctx, "alice@example.com", "Alice", "x", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.Store.UpdateUser(ctx, u.ID, u.Name, u.Role, true); err != nil {
		t.Fatal(err)
	}
	wantErr(t, e.login(e.browser()), "user_disabled")
}

func TestSSOProvisioningOff(t *testing.T) {
	e := newSSOEnv(t, func(s *sso.Settings) { s.AutoProvision = false })
	wantErr(t, e.login(e.browser()), "not_provisioned")
	// an existing user may still sign in
	if _, err := e.s.Store.CreateUser(context.Background(), "alice@example.com", "Alice", "x", "viewer"); err != nil {
		t.Fatal(err)
	}
	c := e.browser()
	if loc := e.login(c); loc != "/" {
		t.Fatalf("existing user rejected: %q", loc)
	}
	if u := e.me(c); u == nil || u.Role != "admin" { // mapped from corvus-admins
		t.Fatalf("me = %+v", u)
	}
}

func TestSSODenyWithoutGroup(t *testing.T) {
	e := newSSOEnv(t, func(s *sso.Settings) { s.DefaultRole = "deny" })
	e.identity("carol@example.com", "unrelated")
	wantErr(t, e.login(e.browser()), "no_role")
}

func TestSSODisabledSettings(t *testing.T) {
	e := newSSOEnv(t, func(s *sso.Settings) { s.Enabled = false })
	resp := e.get(e.browser(), e.console.URL+"/api/auth/oidc/start")
	wantErr(t, resp.Header.Get("Location"), "disabled")
	r, _ := http.Get(e.console.URL + "/api/auth/sso")
	var pub map[string]any
	_ = json.NewDecoder(r.Body).Decode(&pub)
	r.Body.Close()
	if pub["enabled"] != false {
		t.Fatalf("public sso = %v", pub)
	}
}

func TestSSOSettingsAPI(t *testing.T) {
	e := newSSOEnv(t, nil)
	ctx := context.Background()
	root, _ := e.s.Store.UserByEmail(ctx, "root@example.com")
	tok := auth.RandomToken(32)
	if err := e.s.Store.CreateSession(ctx, auth.TokenHash(tok), root.ID, time.Hour, "127.0.0.1", "test"); err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body any) (int, map[string]any) {
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, e.console.URL+path, rd)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: tok})
		req.Header.Set("X-Corvus-Request", "1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out := map[string]any{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	code, v := call("GET", "/api/settings/sso", nil)
	if code != 200 || v["client_secret_set"] != true || v["client_secret"] != nil || !strings.HasSuffix(v["redirect_uri"].(string), "/api/auth/oidc/callback") {
		t.Fatalf("GET = %d %v", code, v)
	}
	// update without a secret keeps the stored one; scopes are normalised
	in := map[string]any{"enabled": true, "display_name": "Okta", "issuer": e.idp.srv.URL, "client_id": e.idp.clientID,
		"scopes": []string{"email profile groups"}, "allowed_domains": []string{"@Example.com"}, "role_claim": "groups",
		"admin_values": []string{"corvus-admins"}, "operator_values": []string{}, "default_role": "viewer", "auto_provision": true}
	code, v = call("PUT", "/api/settings/sso", in)
	if code != 200 || v["client_secret_set"] != true || v["display_name"] != "Okta" {
		t.Fatalf("PUT = %d %v", code, v)
	}
	if got := v["scopes"].([]any); len(got) != 4 || got[0] != "openid" {
		t.Fatalf("scopes = %v", got)
	}
	if got := v["allowed_domains"].([]any); len(got) != 1 || got[0] != "example.com" {
		t.Fatalf("domains = %v", got)
	}
	raw, _ := e.s.Store.GetMeta(ctx, sso.MetaKey)
	if strings.Contains(raw, e.idp.secret) {
		t.Fatal("client secret stored in clear text")
	}
	// the secret still works after the update
	if loc := e.login(e.browser()); loc != "/" {
		t.Fatalf("login after update: %q", loc)
	}
	// unreachable / bad issuers are rejected
	in["issuer"] = "http://idp.example.com"
	if code, _ := call("PUT", "/api/settings/sso", in); code != 422 {
		t.Fatalf("http issuer accepted: %d", code)
	}
	in["issuer"] = e.idp.srv.URL + "/nope"
	if code, _ := call("PUT", "/api/settings/sso", in); code != 422 {
		t.Fatalf("bad discovery accepted: %d", code)
	}
	if code, v := call("POST", "/api/settings/sso/test", map[string]string{"issuer": e.idp.srv.URL}); code != 200 || v["ok"] != true {
		t.Fatalf("test discovery = %d %v", code, v)
	}
	r, _ := http.Get(e.console.URL + "/api/auth/sso")
	var pub map[string]any
	_ = json.NewDecoder(r.Body).Decode(&pub)
	r.Body.Close()
	if pub["enabled"] != true || pub["display_name"] != "Okta" || len(pub) != 2 {
		t.Fatalf("public sso = %v", pub)
	}
}

func TestSSOClientGuards(t *testing.T) {
	for _, bad := range []string{"http://idp.example.com", "ftp://x", "https://u:p@idp.example.com", "https://idp.example.com/?a=1", "idp.example.com"} {
		if sso.CheckIssuer(bad) == nil {
			t.Errorf("issuer %q accepted", bad)
		}
	}
	for _, ok := range []string{"https://idp.example.com", "https://login.example.com/realms/corp", "http://localhost:8080", "http://127.0.0.1:5556/dex"} {
		if err := sso.CheckIssuer(ok); err != nil {
			t.Errorf("issuer %q rejected: %v", ok, err)
		}
	}
	// a public issuer must not be able to point endpoints at loopback / metadata addresses
	c := sso.NewHTTPClient("https://idp.example.com")
	for _, u := range []string{"http://127.0.0.1/", "https://127.0.0.1/", "https://169.254.169.254/latest/meta-data"} {
		if resp, err := c.Get(u); err == nil {
			resp.Body.Close()
			t.Errorf("request to %s allowed", u)
		}
	}
}
