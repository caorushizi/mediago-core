package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestAES128Decryptor_WrongKey(t *testing.T) {
	// Encrypt with one key, decrypt with another → should fail unpadding
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("hello world12345") // exactly 16 bytes

	// Encrypt
	block, _ := aes.NewCipher(key)
	padded := append(plaintext, make([]byte, 16)...)
	for i := 16; i < 32; i++ {
		padded[i] = 16
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, 32)
	mode.CryptBlocks(encrypted, padded)

	// Decrypt with wrong key
	wrongKey := []byte("fedcba9876543210")
	d := &AES128Decryptor{}
	_, err := d.Decrypt(encrypted, wrongKey, iv)
	if err == nil {
		t.Error("expected unpadding error when decrypting with wrong key")
	}
}

func TestAES128Decryptor_WrongIV(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("hello world12345")

	block, _ := aes.NewCipher(key)
	padded := append(plaintext, make([]byte, 16)...)
	for i := 16; i < 32; i++ {
		padded[i] = 16
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, 32)
	mode.CryptBlocks(encrypted, padded)

	wrongIV := []byte("9876543210fedcba")
	d := &AES128Decryptor{}
	// With wrong IV, CBC first block is corrupted but padding might still be valid
	// since IV only affects first block. This tests the code path.
	_, _ = d.Decrypt(encrypted, key, wrongIV)
}

func TestPkcs7Unpad_InvalidPaddingByte(t *testing.T) {
	// Data where last byte says padLen=4 but not all pad bytes match
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 3, 3, 3, 4}
	_, err := pkcs7Unpad(data)
	if err == nil {
		t.Error("expected error for invalid padding bytes")
	}
}

func TestPkcs7Unpad_ZeroPadLen(t *testing.T) {
	data := make([]byte, 16)
	data[15] = 0 // padLen = 0
	_, err := pkcs7Unpad(data)
	if err == nil {
		t.Error("expected error for zero padding length")
	}
}

func TestPkcs7Unpad_PadLenExceedsBlockSize(t *testing.T) {
	data := make([]byte, 16)
	data[15] = 17 // padLen > blockSize
	_, err := pkcs7Unpad(data)
	if err == nil {
		t.Error("expected error for padding length exceeding block size")
	}
}

func TestPkcs7Unpad_PadLenExceedsData(t *testing.T) {
	data := []byte{5}
	_, err := pkcs7Unpad(data)
	if err == nil {
		t.Error("expected error for padding length exceeding data length")
	}
}
