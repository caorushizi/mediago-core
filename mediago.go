package main

import (
	"errors"
	"flag"
	"fmt"
	"mediago/engine"
	"mediago/parser"
	"mediago/utils"
	"time"
)

func main() {
	var (
		err      error
		name     string
		videoDir string
		m3u8Url  string
		headers  string
	)

	flag.StringVar(&name, "name", "新影片", "string类型参数")
	flag.StringVar(&videoDir, "path", "", "string类型参数")
	flag.StringVar(&m3u8Url, "url", "", "string类型参数")
	flag.StringVar(&headers, "headers", "", "string类型参数")

	flag.Parse()

	if !utils.IsUrl(m3u8Url) {
		fmt.Printf("参数 url 格式错误\n")
		time.Sleep(10 * time.Second)
		panic(errors.New("参数 url 格式错误"))
	}

	utils.Headers = headers

	videoDir = utils.NormalizePath(videoDir)

	e := &engine.Engine{}

	decoder := &engine.M3u8Decoder{}

	params := engine.DownloadParams{
		Name:     name,
		Local:    videoDir,
		Url:      m3u8Url,
		Decoder:  decoder,
		Download: parser.DownloadSegment,
	}

	if err = e.Run(params); err != nil {
		utils.Logger.Error(err)
		time.Sleep(10 * time.Second)
		panic(err)
	}
}
