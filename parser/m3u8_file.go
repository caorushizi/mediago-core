package parser

import (
	"bytes"
	"errors"
	"github.com/grafov/m3u8"
	"mediago/utils"
)

func ParseM3u8File(m3u8Url string) (playlist *m3u8.MediaPlaylist, err error) {
	// 开始初始化解析器
	utils.Logger.Debugf("初始化解析器")
	var (
		listType m3u8.ListType
		p        m3u8.Playlist
	)

	// 开始处理 http 请求
	utils.Logger.Debugf("开始解析 m3u8 文件")
	var content []byte
	if content, err = utils.HttpGet(m3u8Url); err != nil {
		return
	}

	if p, listType, err = m3u8.DecodeFrom(bytes.NewReader(content), true); err != nil {
		return
	}

	switch listType {
	case m3u8.MEDIA:
		playlist = p.(*m3u8.MediaPlaylist)
	case m3u8.MASTER:
		return nil, errors.New("不是播放列表")
	}

	if playlist == nil {
		return nil, errors.New("片段列表为空")
	}

	return playlist, err
}
