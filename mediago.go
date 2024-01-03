package main

import (
	"fmt"

	v1 "caorushizi.cn/mediago/api/v1"
	"caorushizi.cn/mediago/config"
	"github.com/gin-gonic/gin"
)

func main() {

	fmt.Printf("Hello, World!%v\n", config.Config)

	router := gin.Default()

	group := router.Group("/v1")
	{
		group.POST("/video/add", v1.AddVideoHandler)
	}

	router.Run()
}
