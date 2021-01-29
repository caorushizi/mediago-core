package main

import (
	"errors"
	"flag"

	"mediago/engine"
	"mediago/utils"
)

func main() {
	var (
		err      error
		name     string
		videoDir string
		m3u8Url  string
	)

	flag.StringVar(&name, "name", "新影片", "string类型参数")
	flag.StringVar(&videoDir, "path", "", "string类型参数")
	flag.StringVar(&m3u8Url, "url", "", "string类型参数")
	flag.Parse()

	if !utils.IsUrl(m3u8Url) {
		panic(errors.New("参数 url 格式错误"))
	}

	videoDir = utils.NormalizePath(videoDir)

	if err = engine.Start(name, videoDir, m3u8Url); err != nil {
		utils.Logger.Error(err)
	}
}
