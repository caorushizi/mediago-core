package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// AES128Decryptor implements AES-128-CBC decryption for HLS segments.
type AES128Decryptor struct{}

// Decrypt decrypts data using AES-128-CBC with PKCS7 unpadding.
func (d *AES128Decryptor) Decrypt(data []byte, key []byte, iv []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("invalid key length: %d, expected 16", len(key))
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("invalid IV length: %d, expected 16", len(iv))
	}
	if len(data) == 0 {
		return nil, nil
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("data length %d is not a multiple of block size %d", len(data), aes.BlockSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)

	decrypted := make([]byte, len(data))
	mode.CryptBlocks(decrypted, data)

	decrypted, err = pkcs7Unpad(decrypted)
	if err != nil {
		return nil, fmt.Errorf("unpad: %w", err)
	}

	return decrypted, nil
}

// pkcs7Unpad removes PKCS7 padding from decrypted data.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding byte at position %d", i)
		}
	}
	return data[:len(data)-padLen], nil
}
