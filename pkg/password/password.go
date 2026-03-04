package password

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

func GeneratePasswordHash(pw string) string {
	salt := GenerateSalt()
	iter := 24000
	keyLen := 32
	encodedPassword := pbkdf2.Key([]byte(pw), []byte(salt), iter, keyLen, sha256.New)
	encoded := base64.StdEncoding.EncodeToString(encodedPassword)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iter, salt, encoded)
}

func MatchPassword(passwordHash, pw string) error {
	parts := strings.Split(passwordHash, "$")
	if parts == nil || len(parts) != 4 {
		return errors.New("invalid password hash")
	}
	if parts[0] != "pbkdf2_sha256" {
		return errors.New("invalid password hash")
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	salt := []byte(parts[2])
	hash := parts[3]
	b, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return err
	}
	dk := pbkdf2.Key([]byte(pw), salt, iter, sha256.Size, sha256.New)
	if !bytes.Equal(b, dk) {
		return errors.New("incorrect password")
	}
	return nil
}

func ValidatePassword(pw string, minLength int) error {
	if len(pw) < minLength {
		return fmt.Errorf("password must be at least %d characters", minLength)
	}
	return nil
}

func CheckMatchingPasswords(pw, confirmation string) error {
	if pw != confirmation {
		return errors.New("passwords do not match")
	}
	return nil
}
