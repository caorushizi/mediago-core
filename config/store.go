package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Store struct {
	Local string `json:"local"`
}

const filepath = "C:\\Users\\84996\\AppData\\Roaming\\media-downloader\\config.json"

var Config Store

func init() {
	// 打开文件
	file, _ := os.Open(filepath)

	// 关闭文件
	defer file.Close()

	//NewDecoder创建一个从file读取并解码json对象的*Decoder，解码器有自己的缓冲，并可能超前读取部分json数据。
	//Decode从输入流读取下一个json编码值并保存在v指向的值里
	err := json.NewDecoder(file).Decode(&Config)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return
}
