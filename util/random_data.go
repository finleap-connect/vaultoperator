package util

import (
	"math/rand"
	"time"

	"github.com/sethvargo/go-password/password"
)

const allowedGlyphs = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.,/?!@#$%^&*()_+}|\":;'\\][=-"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = allowedGlyphs[rand.Intn(len(allowedGlyphs))]
	}
	return b
}

func RandString(n int) string {
	return string(RandBytes(n))
}

func RandPassword(n, digits, symbols int) (string, error) {
	return password.Generate(n, digits, symbols, false, true)
}
