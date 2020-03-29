package main

import (
	"flag"
	"fmt"
	"log"
	"mediago/downloader"
	"mediago/m3u8"
	"mediago/scheduler"
	"mediago/utils"
	"os"
	"path"
)

func Start(nameString, pathString, urlString string) {
	var err error

	if err = utils.CheckDirAndAccess(pathString); err != nil {
		panic(err)
	}

	var (
		sc       scheduler.Scheduler
		playlist *m3u8.ExtM3u8
	)
	// 开始初始化下载器
	log.Println("初始化下载器。")
	if sc, err = scheduler.New(15); err != nil {
		panic(err)
	}
	// 开始初始化解析器
	log.Println("初始化解析器。")
	if playlist, err = m3u8.New(nameString, urlString); err != nil {
		panic(err)
	}

	log.Println("开始解析 m3u8 文件。")
	if err = playlist.Parse(); err != nil {
		panic(err)
	}

	var (
		baseMediaDir   string // 分片文件文件夹下载路径
		segmentDirName string // 分片文件目录名称
		segmentDir     string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
		currentPart    *m3u8.ExtM3u8
	)
	// 创建视频集合文件夹
	baseMediaDir = path.Join(pathString, nameString)
	if err = os.MkdirAll(baseMediaDir, os.ModePerm); err != nil {
		panic(err)
	}

	// 创建视频文件夹
	if len(playlist.Parts) > 0 {
		// fixme: 指定下载通道
		currentPart = playlist.Parts[0]
		segmentDirName = playlist.Parts[0].Name
	} else {
		currentPart = playlist
		segmentDirName = "part_1"
	}

	segmentDir = path.Join(baseMediaDir, segmentDirName)
	if err = os.MkdirAll(segmentDir, os.ModePerm); err != nil {
		panic(err)
	}

	// 分发的下载线程
	go func() {
		for index, segmentUrl := range currentPart.Segments {
			sc.Chs <- 1 // 限制线程数 （每次下载缓存加1， 直到加满阻塞）
			sc.Add(1)

			filename := fmt.Sprintf("%04d.ts", index)
			filePath := path.Join(segmentDir, filename)

			go func(localPath string, urlString string) {
				if err = sc.Work(func() (err error) {
					// 处理下载文件路径
					if err = downloader.StartDownload(filePath, urlString); err != nil {
						return
					}
					return
				}); err != nil {
					// 出现错误 输出错误
					log.Println("下载时出错：", err)
				}
			}(pathString, segmentUrl.Url.String())
		}
		sc.Wait()     // 等待所有分发出去的线程结束
		close(sc.Ans) // 否则 range 会报错哦
	}()

	// 静静的等待每个下载完成
	for range sc.Ans {
		sc.Success++
		fmt.Printf("总共%d个，已经下载%d个~\n", len(currentPart.Segments), sc.Success)
	}

	outFile := path.Join(pathString, fmt.Sprintf("%s.mp4", nameString))
	if err = utils.ConcatVideo(segmentDir, outFile); err != nil {
		log.Println("合并文件出错：", err)
	}

	fmt.Println("完成下载")
}

func main() {
	var name = flag.String("name", "新影片", "string类型参数")
	var pathString = flag.String("path", "", "string类型参数")
	var url = flag.String("url", "", "string类型参数")
	flag.Parse()

	Start(*name, *pathString, *url)
}
