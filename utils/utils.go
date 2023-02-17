package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"regexp"
	"time"

	"github.com/grafov/m3u8"
)

var (
	urlReg    = regexp.MustCompile("^https?://")
	CachedKey = make(map[*m3u8.Key][]byte)
	Headers   string
)

func IsUrl(str string) bool {
	return urlReg.MatchString(str)
}

func AES128Decrypt(encrypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(encrypted))
	blockMode.CryptBlocks(origData, encrypted)
	origData = pkcs5UnPadding(origData)

	for index := range origData {
		if origData[index] == uint8(71) {
			origData = origData[index:]
			break
		}
	}

	return origData, nil
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

func ParseKeyFromUrl(key *m3u8.Key, baseUrl string) {
	// 如果已经请求过了，就不在请求
	if _, ok := CachedKey[key]; ok {
		return
	}

	if key.URI != "" {
		keyUrl, err := ResolveUrl(key.URI, baseUrl)
		if err != nil {
			fmt.Printf("解析KEY时出错\n")
			time.Sleep(10 * time.Second)
			panic(err)
		}
		content, err := HttpGet(keyUrl)
		if err != nil {
			fmt.Printf("解析KEY时出错\n")
			time.Sleep(10 * time.Second)
			panic(err)
		}

		CachedKey[key] = content
	}
}
