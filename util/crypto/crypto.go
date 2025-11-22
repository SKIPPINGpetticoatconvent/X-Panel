package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"golang.org/x/crypto/bcrypt"
)

func HashPasswordAsBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPasswordHash(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckPassword(password, hash string) error {
	// First try bcrypt hash check
	if CheckPasswordHash(hash, password) {
		return nil
	}
	// If bcrypt fails, check if it's plain text (this indicates migration needed)
	// For plain text, we assume it's correct if lengths match and no special hash chars
	if len(hash) > 0 && hash == password {
		return nil
	}
	return errors.New("invalid password")
}
func AesDecrypt(base64Ciphertext, key string) ([]byte, error) {
	// Decode base64 encrypted data
	ciphertext, err := base64.StdEncoding.DecodeString(base64Ciphertext)
	if err != nil {
		return nil, err
	}

	// Create AES cipher
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	// Check if ciphertext is long enough
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// Get IV
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove padding
	paddingLen := int(ciphertext[len(ciphertext)-1])
	if paddingLen > aes.BlockSize || paddingLen == 0 {
		return nil, errors.New("invalid padding")
	}
	for i := len(ciphertext) - paddingLen; i < len(ciphertext); i++ {
		if ciphertext[i] != byte(paddingLen) {
			return nil, errors.New("invalid padding")
		}
	}

	return ciphertext[:len(ciphertext)-paddingLen], nil
}
