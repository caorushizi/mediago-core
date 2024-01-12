package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	staticPath string
	videoPath  string
)

type VideoItem struct {
	Name string `json:"name"`
	Url  string `json:"url"`
	Id   string `json:"id"`
}

func GetVideoListHandler(c *gin.Context) {
	// 获取 videoPath 下所有的视频文件
	var files []VideoItem
	err := filepath.Walk(videoPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".mp4") {
			filename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

			files = append(files, VideoItem{
				Name: filename,
				Url:  "/video/" + info.Name(),
				Id:   filename,
			})
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 返回文件列表
	c.JSON(http.StatusOK, files)
}

func GetVideoHandler(c *gin.Context) {
	filename := c.Param("filename")
	fmt.Println(filename)

	c.File(filepath.Join(videoPath, filename))
}

func main() {

	flag.StringVar(&staticPath, "staticPath", "", "静态资源目录")
	flag.StringVar(&videoPath, "videoPath", "", "视频文件目录")

	flag.Parse()

	if staticPath == "" {
		fmt.Fprintln(os.Stderr, "Error: no staticPath provided")
		os.Exit(1)
	}

	fmt.Printf("Hello, %s!\n", staticPath)

	router := gin.Default()

	router.GET("/api/video-list", GetVideoListHandler)
	router.GET("/video/:filename", GetVideoHandler)

	router.NoRoute(func(c *gin.Context) {
		c.File(staticPath + c.Request.URL.Path)
	})

	router.Run()
}
