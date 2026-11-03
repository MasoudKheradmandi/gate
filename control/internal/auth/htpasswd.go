package auth

import (
	"crypto/md5"
	"crypto/rand"
	"strings"
)

// APR1 hashes a password with the Apache "$apr1$" MD5-crypt scheme, which nginx's
// auth_basic verifies natively (independent of the libc crypt implementation).
func APR1(password string) string {
	const alphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	salt := make([]byte, 8)
	for i := range b {
		salt[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return apr1(password, string(salt))
}

func apr1(pw, salt string) string {
	const magic = "$apr1$"
	p := []byte(pw)
	s := []byte(salt)

	alt := md5.Sum(append(append(append([]byte{}, p...), s...), p...))
	ctx := md5.New()
	ctx.Write(p)
	ctx.Write([]byte(magic))
	ctx.Write(s)
	for i := len(p); i > 0; i -= 16 {
		n := i
		if n > 16 {
			n = 16
		}
		ctx.Write(alt[:n])
	}
	for i := len(p); i > 0; i >>= 1 {
		if i&1 == 1 {
			ctx.Write([]byte{0})
		} else {
			ctx.Write(p[:1])
		}
	}
	final := ctx.Sum(nil)
	for i := 0; i < 1000; i++ {
		c := md5.New()
		if i&1 == 1 {
			c.Write(p)
		} else {
			c.Write(final)
		}
		if i%3 != 0 {
			c.Write(s)
		}
		if i%7 != 0 {
			c.Write(p)
		}
		if i&1 == 1 {
			c.Write(final)
		} else {
			c.Write(p)
		}
		final = c.Sum(nil)
	}
	const itoa64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var out strings.Builder
	enc := func(a, b, c byte, n int) {
		v := uint(a)<<16 | uint(b)<<8 | uint(c)
		for ; n > 0; n-- {
			out.WriteByte(itoa64[v&0x3f])
			v >>= 6
		}
	}
	enc(final[0], final[6], final[12], 4)
	enc(final[1], final[7], final[13], 4)
	enc(final[2], final[8], final[14], 4)
	enc(final[3], final[9], final[15], 4)
	enc(final[4], final[10], final[5], 4)
	enc(0, 0, final[11], 2)
	return magic + salt + "$" + out.String()
}
