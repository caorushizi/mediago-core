package parser

import (
	"mediago/utils"
)

func DownloadSegment(remoteUrl string) (content []byte, err error) {

	// 开启 http client 下载文件
	if content, err = utils.HttpGet(remoteUrl); err != nil {
		return nil, err
	}

	return
}
