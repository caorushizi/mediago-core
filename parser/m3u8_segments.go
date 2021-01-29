package parser

import (
	"encoding/hex"
	"fmt"
	"mediago/utils"
	"os"
	"path"

	"github.com/grafov/m3u8"
)

func ParseM3u8Segments(index int, localPath string, baseUrl string, segmentUrl string, key *m3u8.Key) (err error) {
	var (
		fullUrl      string
		downloadFile *os.File
		content      []byte
		filename     = fmt.Sprintf("%06d.ts", index)
		filepath     = path.Join(localPath, filename)
	)

	if fullUrl, err = utils.ResolveUrl(segmentUrl, baseUrl); err != nil {
		return
	}

	// 判断文件是否存在，如果存在则跳过下载
	if utils.FileExist(filepath) {
		return
	}

	// 开启 http client 下载文件
	if content, err = utils.HttpGet(fullUrl); err != nil {
		return
	}

	// 在此对文件进行解码
	if key != nil {
		if key.Method == "AES-128" {
			var iv []byte
			if iv, err = hex.DecodeString(key.IV[2:]); err != nil {
				return
			}

			if content, err = utils.AES128Decrypt(content, utils.CachedKey[key], iv); err != nil {
				return
			}

			syncByte := uint8(71) //0x47
			for index := range content {
				if content[index] == syncByte {
					content = content[index:]
					break
				}
			}
		}
	}

	if downloadFile, err = os.Create(filepath + ".tmp"); err != nil {
		return
	}

	_, err = downloadFile.Write(content)
	err = downloadFile.Close()
	if err != nil {
		return
	}

	if err = os.Rename(filepath+".tmp", filepath); err != nil {
		return err
	}

	return
}
