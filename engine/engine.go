package engine

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/grafov/m3u8"

	"mediago/parser"
	"mediago/scheduler"
	"mediago/utils"
)

var (
	sc scheduler.Scheduler
)

func Start(nameString, pathString, urlString string) (err error) {
	outFile := path.Join(pathString, fmt.Sprintf("%s.mp4", nameString))
	if utils.FileExist(outFile) {
		return errors.New("文件已经存在！")
	}

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")
	sc = scheduler.New(15)

	if err = utils.CheckDirAndAccess(pathString); err != nil {
		return
	}

	var (
		playlist       *m3u8.MediaPlaylist
		baseMediaDir   string // 分片文件文件夹下载路径
		segmentDirName string // 分片文件目录名称
		segmentDir     string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
	)

	if playlist, err = parser.ParseM3u8File(urlString); err != nil {
		return
	}

	// 创建视频集合文件夹
	baseMediaDir = path.Join(pathString, nameString)
	if err = os.MkdirAll(baseMediaDir, os.ModePerm); err != nil {
		return
	}

	// 创建视频片段文件夹
	segmentDirName = "part_1"
	segmentDir = path.Join(baseMediaDir, segmentDirName)
	if err = os.MkdirAll(segmentDir, os.ModePerm); err != nil {
		return
	}

	// 分发的下载线程
	go dispatchTask(segmentDir, urlString, playlist)

	// 静静的等待每个下载完成
	for filename := range sc.Ans {
		sc.Success++
		utils.Logger.Infof("%06d: %06d 片段已完成", sc.Success, filename)
	}

	if err = utils.ConcatVideo(segmentDir, outFile); err != nil {
		return fmt.Errorf("合并文件出错：%s", err)
	}

	utils.Logger.Infof("下载完成")
	return
}

func dispatchTask(localPath, baseUrl string, playlist *m3u8.MediaPlaylist) {
	var segments []m3u8.MediaSegment
	for _, segment := range playlist.Segments {
		if segment != nil {
			segments = append(segments, *segment)
		}
	}
	utils.Logger.Debugf("总共有 %d 条任务", len(segments))

	var (
		segmentKey *m3u8.Key
	)

	for index, segment := range segments {

		var (
			segmentURI string
		)

		// 当前片段是否含有 key ，如果没有则使用上一个片段的 key
		if segment.Key != nil {
			segmentKey = segment.Key
			utils.ParseKeyFromUrl(segmentKey, baseUrl)
		}

		segmentURI = segment.URI
		go execute(index, localPath, baseUrl, segmentURI, segmentKey)
	}

	sc.Wait()
	close(sc.Ans)
}

func execute(index int, localPath string, baseUrl string, segmentUrl string, key *m3u8.Key) {
	sc.Work(index, func() (err error) {
		err = parser.ParseM3u8Segments(index, localPath, baseUrl, segmentUrl, key)
		return
	})
}
