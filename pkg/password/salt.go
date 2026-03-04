package password

import (
	"math/rand"
	"time"
)

const (
	digits           = "0123456789"
	hexLetters       = "abcdef"
	lowerCaseLetters = "abcdefghijklmnopqrstuvwxyz"
	upperCaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func GenerateSalt() string {
	return generateRandomString(digits+lowerCaseLetters+upperCaseLetters, 12)
}

func generateRandomString(charset string, length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
