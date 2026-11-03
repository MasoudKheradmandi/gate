package auth

import "golang.org/x/crypto/bcrypt"

// htpasswdCost is the bcrypt cost for gateway password protection (nginx auth_basic).
// nginx verifies the hash on every request that carries credentials, so the cost is
// kept low enough not to turn each page load into a CPU burst (~2-5 ms per check),
// while still being far stronger than MD5-based apr1 if the file ever leaks.
// Verification uses the C library's crypt(); musl supports $2a$/$2b$/$2y$.
const htpasswdCost = 6

// HtpasswdHash hashes a password for an nginx htpasswd file (bcrypt). Hashes created by
// earlier versions ($apr1$) stay valid: nginx verifies both.
func HtpasswdHash(password string) (string, error) {
	if len(password) > 72 {
		password = password[:72] // bcrypt input limit; callers validate length first
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), htpasswdCost)
	return string(b), err
}
