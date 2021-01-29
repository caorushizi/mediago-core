package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/grafov/m3u8"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var (
	urlReg    = regexp.MustCompile("^https?://")
	CachedKey = make(map[*m3u8.Key][]byte)
)

func CheckDirAndAccess(pathString string) (err error) {
	// 检查下载路径是否存在
	// 并且检查时候有权限写入文件
	fileInfo, err := os.Stat(pathString)
	if err != nil && os.IsNotExist(err) && !fileInfo.IsDir() {
		return
	}
	// fixme： 检查时候有权限写入
	return
}

func FileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func IsUrl(str string) bool {
	return urlReg.MatchString(str)
}

func ConcatVideo(segmentDir, outFile string) (err error) {
	// 生成列表文件
	var (
		fileListTextPath string // 分片文件检索文件 = baseMediaPath + "filelist.txt"
		fileListContent  string // 分片文件检索文件内容
		fileListFiles    []os.FileInfo
		playlistFile     *os.File
		mediaDir         string
	)

	// 在分片视频文件夹同级创建 filelist.txt
	mediaDir = path.Dir(segmentDir)
	fileListTextPath = path.Join(mediaDir, "filelist.txt")
	if playlistFile, err = os.Create(fileListTextPath); err != nil {
		return
	}

	if fileListFiles, err = ioutil.ReadDir(segmentDir); err != nil {
		return
	}

	for _, segmentFile := range fileListFiles {
		if strings.HasSuffix(segmentFile.Name(), ".ts") {
			filePath := path.Join(path.Base(segmentDir), segmentFile.Name())
			fileListItem := fmt.Sprintf("file '%s'\n", filePath)
			fileListContent += fileListItem
		}
	}

	if _, err = playlistFile.WriteString(fileListContent); err != nil {
		return
	}

	// 开始执行命令
	var cmd *exec.Cmd

	cmd = exec.Command("ffmpeg", "-f", "concat", "-i", fileListTextPath,
		"-acodec", "copy", "-vcodec", "copy", outFile)

	if _, err = cmd.CombinedOutput(); err != nil {
		fmt.Print(cmd.Stderr)
		return
	}
	return
}

func AES128Decrypt(crypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = pkcs5UnPadding(origData)
	return origData, nil
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

func ResolveUrl(segmentText string, baseUrl string) (urlString string, err error) {
	// 如果 m3u8 列表中已经是完整的 url
	if IsUrl(segmentText) {
		return segmentText, nil
	}

	var resultUrl *url.URL
	if resultUrl, err = url.Parse(baseUrl); err != nil {
		return "", err
	}
	if path.IsAbs(segmentText) {
		resultUrl.Path = segmentText
	} else {
		tempBaseUrl := path.Dir(resultUrl.Path)
		resultUrl.Path = path.Join(tempBaseUrl, segmentText)
	}

	return resultUrl.String(), nil
}

func ParseKeyFromUrl(key *m3u8.Key, baseUrl string) {
	// 如果已经请求过了，就不在请求
	if _, ok := CachedKey[key]; ok {
		return
	}

	if key.URI != "" {
		keyUrl, err := ResolveUrl(key.URI, baseUrl)
		if err != nil {
			panic(err)
		}
		content, err := HttpGet(keyUrl)
		if err != nil {
			panic(err)
		}

		CachedKey[key] = content
	}
}
