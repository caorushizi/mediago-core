package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// helper: encrypt with AES-128-CBC + PKCS7 padding
func aesEncrypt(plaintext, key, iv []byte) []byte {
	// PKCS7 pad
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	block, _ := aes.NewCipher(key)
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)
	return encrypted
}

func TestAES128Decryptor_Decrypt(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	iv := []byte("abcdef0123456789")  // 16 bytes
	plaintext := []byte("hello world, this is a test segment content!")

	encrypted := aesEncrypt(plaintext, key, iv)

	dec := &AES128Decryptor{}
	result, err := dec.Decrypt(encrypted, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Errorf("decrypted mismatch:\ngot:  %q\nwant: %q", result, plaintext)
	}
}

func TestAES128Decryptor_ExactBlockSize(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	// Exactly 16 bytes — padding will add a full block
	plaintext := []byte("exactly16bytes!!")

	encrypted := aesEncrypt(plaintext, key, iv)

	dec := &AES128Decryptor{}
	result, err := dec.Decrypt(encrypted, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result, plaintext) {
		t.Errorf("decrypted mismatch:\ngot:  %q\nwant: %q", result, plaintext)
	}
}

func TestAES128Decryptor_InvalidKeyLength(t *testing.T) {
	dec := &AES128Decryptor{}
	_, err := dec.Decrypt(make([]byte, 16), []byte("short"), make([]byte, 16))
	if err == nil {
		t.Error("expected error for invalid key length")
	}
}

func TestAES128Decryptor_InvalidIVLength(t *testing.T) {
	dec := &AES128Decryptor{}
	_, err := dec.Decrypt(make([]byte, 16), make([]byte, 16), []byte("short"))
	if err == nil {
		t.Error("expected error for invalid IV length")
	}
}

func TestAES128Decryptor_EmptyData(t *testing.T) {
	dec := &AES128Decryptor{}
	result, err := dec.Decrypt(nil, make([]byte, 16), make([]byte, 16))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestAES128Decryptor_InvalidDataLength(t *testing.T) {
	dec := &AES128Decryptor{}
	_, err := dec.Decrypt(make([]byte, 15), make([]byte, 16), make([]byte, 16))
	if err == nil {
		t.Error("expected error for non-block-aligned data")
	}
}
