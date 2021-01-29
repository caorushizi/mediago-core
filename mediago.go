package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/grafov/m3u8"
	"net/url"
	"os"
	"path"

	"mediago/scheduler"
	"mediago/utils"
)

var (
	sc        scheduler.Scheduler
	cachedKey = make(map[*m3u8.Key][]byte)
)

func Start(nameString, pathString, urlString string) {
	outFile := path.Join(pathString, fmt.Sprintf("%s.mp4", nameString))
	if utils.FileExist(outFile) {
		panic(errors.New("文件已经存在！"))
	}

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")
	sc = scheduler.New(15)

	var err error

	if err = utils.CheckDirAndAccess(pathString); err != nil {
		panic(err)
	}

	// 开始初始化解析器
	utils.Logger.Debugf("初始化解析器")
	var (
		url1     *url.URL
		playlist *m3u8.MediaPlaylist
	)
	if url1, err = url.Parse(urlString); err != nil {
		return
	}
	// 开始处理 http 请求
	utils.Logger.Debugf("开始解析 m3u8 文件")
	var content []byte
	if content, err = utils.HttpGet(url1.String()); err != nil {
		return
	}

	p, listType, err := m3u8.DecodeFrom(bytes.NewReader(content), true)
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

	if playlist == nil {
		return
	}

	var segments []m3u8.MediaSegment
	for _, segment := range playlist.Segments {
		if segment != nil {
			segments = append(segments, *segment)
		}
	}
	utils.Logger.Debugf("总共有 %d 条任务", len(segments))

	// 分发的下载线程
	go dispatchTask(segmentDir, url1.String(), segments)

	// 静静的等待每个下载完成
	for filename := range sc.Ans {
		sc.Success++
		percentage := float32(sc.Success) / float32(len(segments))
		utils.Logger.Infof("%2f %06d/%06d: %06d 片段已完成", percentage, sc.Success, len(segments), filename)
	}

	if err = utils.ConcatVideo(segmentDir, outFile); err != nil {
		utils.Logger.Errorf("合并文件出错：%s", err)
	}

	utils.Logger.Infof("下载完成")
}

func handleDownload(index int, localPath string, baseUrl string, segmentUrl string, key *m3u8.Key) {

	sc.Work(index, func() (err error) {
		err = fetch(index, localPath, baseUrl, segmentUrl, key)
		return
	})
}

func dispatchTask(localPath, baseUrl string, segments []m3u8.MediaSegment) {
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
			parseKeyFromUrl(segmentKey, baseUrl)
		}

		segmentURI = segment.URI
		go handleDownload(index, localPath, baseUrl, segmentURI, segmentKey)
	}

	sc.Wait()
	close(sc.Ans)
}

func parseKeyFromUrl(key *m3u8.Key, baseUrl string) {
	// 如果已经请求过了，就不在请求
	if _, ok := cachedKey[key]; ok {
		return
	}

	if key.URI != "" {
		keyUrl, err := handleUrl(key.URI, baseUrl)
		if err != nil {
			panic(err)
		}
		content, err := utils.HttpGet(keyUrl)
		if err != nil {
			panic(err)
		}

		cachedKey[key] = content
	}

}

func fetch(index int, localPath string, baseUrl string, segmentUrl string, key *m3u8.Key) (err error) {

	var (
		fullUrl      string
		downloadFile *os.File
		content      []byte
		filename     = fmt.Sprintf("%06d.ts", index)
		filepath     = path.Join(localPath, filename)
	)

	if fullUrl, err = handleUrl(segmentUrl, baseUrl); err != nil {
		panic(err)
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

			if content, err = utils.AES128Decrypt(content, cachedKey[key], iv); err != nil {
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
