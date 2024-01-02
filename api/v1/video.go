package v1

import (
	"fmt"
	"net/http"

	"caorushizi.cn/mediago/utils"
	"github.com/gin-gonic/gin"
)

type VideoForm struct {
	Url  string `json:"url" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// AddVideoHandler handles the request to add a video.
func AddVideoHandler(c *gin.Context) {
	fmt.Println("Hello, World!")

	form := VideoForm{}
	if err := c.BindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	downloader := utils.NewM3u8Downloader(form.Url, form.Name)

	downloader.Start()

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
