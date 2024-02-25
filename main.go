package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var (
	staticPath string
	videoPath  string
	port       string
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

	// urldecode
	filename, err := url.QueryUnescape(filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}

	c.File(filepath.Join(videoPath, filename))
}

func main() {

	flag.StringVar(&staticPath, "static-path", "", "静态资源目录")
	flag.StringVar(&videoPath, "video-path", "", "视频文件目录")
	flag.StringVar(&port, "port", "8433", "端口号")

	flag.Parse()

	if staticPath == "" || videoPath == "" {
		fmt.Fprintln(os.Stderr, "Error: no \"staticPath\" or \"staticPath\" provided")
		os.Exit(1)
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/api/video-list", GetVideoListHandler)
	router.GET("/video/:filename", GetVideoHandler)
	router.Static("/assets", path.Join(staticPath, "assets"))

	router.NoRoute(func(c *gin.Context) {
		c.File(path.Join(staticPath, "index.html"))
	})

	router.Run(":" + port)
}
