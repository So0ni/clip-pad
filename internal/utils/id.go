package utils

import (
	"crypto/rand"
	"fmt"
)

const Charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateID(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}

	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	for i, b := range random {
		buf[i] = Charset[int(b)%len(Charset)]
	}

	return string(buf), nil
}
