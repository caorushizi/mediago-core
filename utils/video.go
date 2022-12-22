package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
)

func ConcatVideo(basePath, filename, part string) (err error) {
	segmentDir := PathJoin(basePath, filename, part)
	mediaDir := PathJoin(basePath, filename)

	// 生成列表文件
	var (
		fileListTextPath string // 分片文件检索文件 = baseMediaPath + "filelist.txt"
		fileListContent  string // 分片文件检索文件内容
		fileListFiles    []os.FileInfo
		playlistFile     *os.File
	)

	// 在分片视频文件夹同级创建 filelist.txt
	fileListTextPath = PathJoin(mediaDir, "filelist.txt")
	if playlistFile, err = os.Create(fileListTextPath); err != nil {
		return
	}
	defer playlistFile.Close()

	if fileListFiles, err = ioutil.ReadDir(segmentDir); err != nil {
		return
	}

	for _, segmentFile := range fileListFiles {
		if strings.HasSuffix(segmentFile.Name(), ".ts") {
			filePath := path.Join(part, segmentFile.Name())
			fileListItem := fmt.Sprintf("file '%s'\n", filePath)
			fileListContent += fileListItem
		}
	}

	if _, err = playlistFile.WriteString(fileListContent); err != nil {
		return
	}

	// 开始执行命令
	var cmd *exec.Cmd

	localPath, _ := os.Getwd()

	binName := "ffmpeg"

	cmd = exec.Command(
		binName,
		"-f",
		"concat",
		"-i",
		fileListTextPath,
		"-acodec",
		"copy",
		"-vcodec",
		"copy",
		PathJoin(basePath, filename+".mp4"),
	)

	cmd.Dir = localPath

	if _, err = cmd.CombinedOutput(); err != nil {
		fmt.Print(cmd.Stderr)
		return
	}
	return
}
