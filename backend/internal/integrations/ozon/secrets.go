package ozon

import (
	"fmt"

	appCrypto "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/crypto"
)

type SecretCodec struct {
	box *appCrypto.SecretBox
}

func NewSecretCodec(encryptionKey string) (*SecretCodec, error) {
	box, err := appCrypto.NewSecretBox(encryptionKey)
	if err != nil {
		return nil, err
	}

	return &SecretCodec{
		box: box,
	}, nil
}

func (c *SecretCodec) Encrypt(value string) (string, error) {
	encrypted, err := c.box.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %w", err)
	}
	return encrypted, nil
}

func (c *SecretCodec) Decrypt(value string) (string, error) {
	decrypted, err := c.box.Decrypt(value)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return decrypted, nil
}
