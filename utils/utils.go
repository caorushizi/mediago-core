package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var (
	urlReg = regexp.MustCompile("^https?://")
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
