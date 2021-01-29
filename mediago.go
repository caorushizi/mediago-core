package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/grafov/m3u8"
	"io"
	"log"
	"mediago/downloader"
	"mediago/scheduler"
	"mediago/utils"
	"net/url"
	"os"
	"path"
)

var sc scheduler.Scheduler

func Start(nameString, pathString, urlString string) {
	outFile := path.Join(pathString, fmt.Sprintf("%s.mp4", nameString))
	if utils.FileExist(outFile) {
		panic(errors.New("文件已经存在！"))
	}

	// 开始初始化下载器
	log.Println("初始化下载器。")
	sc = scheduler.New(15)

	var err error

	if err = utils.CheckDirAndAccess(pathString); err != nil {
		panic(err)
	}

	// 开始初始化解析器
	log.Println("初始化解析器。")
	var (
		url1     *url.URL
		playlist *m3u8.MediaPlaylist
	)
	if url1, err = url.Parse(urlString); err != nil {
		return
	}
	// 开始处理 http 请求
	log.Println("开始解析 m3u8 文件。")
	var repReader io.ReadCloser
	if repReader, err = utils.HttpClient(url1.String()); err != nil {
		return
	}
	defer repReader.Close()

	p, listType, err := m3u8.DecodeFrom(repReader, true)
	if err != nil {
		panic(err)
	}
	switch listType {
	case m3u8.MEDIA:
		playlist = p.(*m3u8.MediaPlaylist)
	case m3u8.MASTER:
		masterPlaylist := p.(*m3u8.MasterPlaylist)
		fmt.Printf("%+v\n", masterPlaylist)
		panic(errors.New("不是播放列表"))
	}

	var (
		baseMediaDir   string // 分片文件文件夹下载路径
		segmentDirName string // 分片文件目录名称
		segmentDir     string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
	)
	// 创建视频集合文件夹
	baseMediaDir = path.Join(pathString, nameString)
	if err = os.MkdirAll(baseMediaDir, os.ModePerm); err != nil {
		panic(err)
	}

	segmentDirName = "part_1"

	segmentDir = path.Join(baseMediaDir, segmentDirName)
	if err = os.MkdirAll(segmentDir, os.ModePerm); err != nil {
		panic(err)
	}

	// 分发的下载线程
	go dispatchTask(segmentDir, url1.String(), playlist)

	// 静静的等待每个下载完成
	for filename := range sc.Ans {
		sc.Success++
		fmt.Printf("%06d/%06d: %06d 片段已完成 \n", sc.Success, len(playlist.Segments), filename)
	}

	if err = utils.ConcatVideo(segmentDir, outFile); err != nil {
		log.Println("合并文件出错：", err)
	}

	fmt.Println("完成下载")
}

func handleDownload(index int, localPath string, baseUrl string, segmentUrl string) {
	filename := fmt.Sprintf("%06d.ts", index)
	filePath := path.Join(localPath, filename)

	var (
		fullUrl string
		err     error
	)
	if fullUrl, err = handleUrl(segmentUrl, baseUrl); err != nil {
		panic(err)
	}

	// 判断文件是否存在，如果存在则跳过下载
	if utils.FileExist(filePath) {
		return
	}

	sc.Work(index, func() (err error) {
		err = downloader.StartDownload(filePath, fullUrl)
		return
	})
}

func dispatchTask(localPath, baseUrl string, playlist *m3u8.MediaPlaylist) {
	for index, m3uSegment := range playlist.Segments {
		if m3uSegment == nil {
			continue
		}

		segmentUrl := m3uSegment.URI
		go handleDownload(index, localPath, baseUrl, segmentUrl)
	}

	sc.Wait()
	close(sc.Ans)
}

func handleUrl(segmentText string, baseUrl string) (urlString string, err error) {
	// 如果 m3u8 列表中已经是完整的 url
	if utils.IsUrl(segmentText) {
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

func main() {
	var name = flag.String("name", "新影片", "string类型参数")
	var pathString = flag.String("path", "", "string类型参数")
	var url = flag.String("url", "", "string类型参数")
	flag.Parse()

	Start(*name, *pathString, *url)
}
