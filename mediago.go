package main

import (
	"errors"
	"flag"

	"mediago/engine"
	"mediago/utils"
)

func main() {
	var nameFlag = flag.String("name", "新影片", "string类型参数")
	var pathFlag = flag.String("path", "", "string类型参数")
	var urlFlag = flag.String("url", "", "string类型参数")
	flag.Parse()

	if !utils.IsUrl(*urlFlag) {
		panic(errors.New("参数 url 格式错误"))
	}

	var err error
	if err = engine.Start(*nameFlag, *pathFlag, *urlFlag); err != nil {
		utils.Logger.Error(err)
	}
}
