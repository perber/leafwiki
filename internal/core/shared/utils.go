package shared

import (
	"crypto/rand"
	"math/big"

	"github.com/teris-io/shortid"
)

// GenerateUniqueID generates a unique ID for a tree entry
func GenerateUniqueID() (string, error) {
	id, err := shortid.Generate()
	if err != nil {
		return "", err
	}

	return id, nil
}

var charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

func GenerateRandomPassword(length int) (string, error) {
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}
