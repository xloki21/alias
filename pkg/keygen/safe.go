package keygen

import (
	"crypto/rand"
	"encoding/base64"
)

type URLSafeRandomStringGenerator struct {
}

func NewURLSafeRandomStringGenerator() *URLSafeRandomStringGenerator {
	return new(URLSafeRandomStringGenerator)
}

func (g *URLSafeRandomStringGenerator) Generate(n int) (string, error) {
	b, err := generateRandomBytes(n)
	return base64.URLEncoding.EncodeToString(b), err
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
