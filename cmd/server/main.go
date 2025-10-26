package main

import (
	"flag"
	"fmt"

	"caorushizi.cn/mediago/internal/serverapp"
)

func main() {
	configJSON := flag.String("config", "", "JSON string with server configuration")
	flag.Parse()

	if err := serverapp.Run(*configJSON); err != nil {
		panic(fmt.Sprintf("server failed to start: %v", err))
	}
}
