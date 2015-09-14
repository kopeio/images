package utils

import (
	crypto_rand "crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/kopeio/kope/chained"
)

// 0-9 and A-Z, but with 1,I and 0,O removed.
// We keep L because we are all upper case
const passwordChars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

var passwordEncoder = base32.NewEncoding(passwordChars)

func GeneratePassword(bits int) (string, error) {
	byteCount := (bits + 7) / 8

	secretBytes := make([]byte, byteCount)
	_, err := crypto_rand.Read(secretBytes)
	if err != nil {
		return "", chained.Error(err, "error generating secret")
	}

	s := passwordEncoder.EncodeToString(secretBytes)
	s = strings.Replace(s, "=", "", -1)
	return s, nil
}
