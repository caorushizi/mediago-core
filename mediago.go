package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"mediago/downloader"
	"mediago/m3u8"
	"mediago/scheduler"
	"mediago/utils"
)

func Start(nameString, pathString, urlString string) {
	var err error

	if err = utils.CheckDirAndAccess(pathString); err != nil {
		panic(err)
	}

	var (
		sc           scheduler.Scheduler
		playlist     m3u8.ExtM3u8
		playlistFile *os.File
	)
	// 开始初始化下载器
	if sc, err = scheduler.New(15); err != nil {
		panic(err)
	}
	// 开始初始化解析器
	if playlist, err = m3u8.New(nameString, urlString); err != nil {
		panic(err)
	}

	if err = playlist.Parse(); err != nil {
		panic(err)
	}

	// 创建视频文件夹
	baseMediaPath := path.Join(pathString, nameString)
	if err = os.MkdirAll(baseMediaPath, os.ModePerm); err != nil {
		panic(err)
	}

	// 生成列表文件
	fileList := path.Join(pathString, nameString, "fileList.txt")
	if playlistFile, err = os.Create(fileList); err != nil {
		panic(err)
	}

	content := ""
	for index := range playlist.Segments {
		filename := fmt.Sprintf("%04d.ts", index)
		segmentItem := fmt.Sprintf("file 'segments/%s'\n", filename)
		content += segmentItem
	}
	if _, err = playlistFile.WriteString(content); err != nil {
		panic(err)
	}

	baseSegmentPath := path.Join(baseMediaPath, "segments")
	if err = os.MkdirAll(baseSegmentPath, os.ModePerm); err != nil {
		panic(err)
	}

	// 分发的下载线程
	go func() {
		for index, segmentUrl := range playlist.Segments {
			sc.Chs <- 1 // 限制线程数 （每次下载缓存加1， 直到加满阻塞）
			sc.Add(1)

			filename := fmt.Sprintf("%04d.ts", index)
			filePath := path.Join(baseSegmentPath, filename)

			go func(localPath string, urlString string) {
				sc.Work(func() (err error) {
					// 处理下载文件路径
					if err = downloader.StartDownload(filePath, urlString); err != nil {
						return
					}
					return
				})
			}(pathString, segmentUrl.String())
		}
		sc.Wait()     // 等待所有分发出去的线程结束
		close(sc.Ans) // 否则 range 会报错哦
	}()

	// 静静的等待每个下载完成
	for range sc.Ans {
		sc.Success++
		fmt.Printf("总共%d个，已经下载%d个~\n", len(playlist.Segments), sc.Success)
	}

	outFile := path.Join(pathString, fmt.Sprintf("%s.mp4", nameString))
	utils.OutputMp4(fileList, outFile)
	fmt.Println("完成下载")
}

func main() {
	var name = flag.String("name", "新影片", "string类型参数")
	var pathString = flag.String("path", "", "string类型参数")
	var url = flag.String("url", "", "string类型参数")
	flag.Parse()

	Start(*name, *pathString, *url)
}
