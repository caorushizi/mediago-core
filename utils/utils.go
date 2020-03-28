package utils

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

func IsUrl(str string) bool {
	return urlReg.MatchString(str)
}

func OutputMp4(filesListPath string, outFile string) {
	var (
		err    error
		cmd    *exec.Cmd
		output []byte
	)

	cmd = exec.Command("ffmpeg", "-f", "concat", "-i", filesListPath,
		"-acodec", "copy", "-vcodec", "copy", "-absf", "aac_adtstoasc", outFile)

	if output, err = cmd.CombinedOutput(); err != nil {
		fmt.Println(err)
		fmt.Println(cmd.Stderr)
		return
	}
	fmt.Println("Result: " + string(output))
}
