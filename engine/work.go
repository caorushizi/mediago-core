package engine

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"mediago/utils"
	"os"
	"path"
)

func processSegment(params DownloadParams) error {
	var (
		downloadFile *os.File
		filepath     = path.Join(params.Local, params.Name)
		content      []byte
		err          error
	)

	// 判断文件是否存在，如果存在则跳过下载
	if utils.FileExist(filepath) {
		return nil
	}

	if content, err = utils.HttpGet(params.Url); err != nil {
		return err
	}

	decoder := *params.Decoder
	switch decoder.Method {
	case "AES-128":
		if content, err = utils.AES128Decrypt(content, decoder.Key, decoder.Iv); err != nil {
			return err
		}
	}

	if downloadFile, err = os.Create(filepath); err != nil {
		return err
	}

	if _, err = downloadFile.Write(content); err != nil {
		return err
	}

	if err = downloadFile.Close(); err != nil {
		return err
	}

	return nil
}

func processM3u8File(params DownloadParams) ([]DownloadParams, error) {
	var (
		err          error
		playlist     *m3u8.MediaPlaylist
		baseMediaDir string // 分片文件文件夹下载路径
		segmentDir   string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
	)

	outFile := utils.PathJoin(params.Local, fmt.Sprintf("%s.mp4", params.Name))
	if utils.FileExist(outFile) {
		return nil, errors.New("文件已经存在！")
	}

	if err = utils.PrepareDir(params.Local); err != nil {
		return nil, err
	}

	// 开始处理 http 请求
	utils.Logger.Debugf("开始解析 m3u8 文件")
	var (
		listType m3u8.ListType
		p        m3u8.Playlist
	)

	var content []byte
	if content, err = utils.HttpGet(params.Url); err != nil {
		return nil, err
	}
	m3u8Reader := bytes.NewReader(content)
	if p, listType, err = m3u8.DecodeFrom(m3u8Reader, true); err != nil {
		return nil, err
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

	// 创建视频集合文件夹
	baseMediaDir = utils.PathJoin(params.Local, params.Name)
	if err = utils.PrepareDir(baseMediaDir); err != nil {
		return nil, err
	}

	// 创建视频片段文件夹
	segmentDir = utils.PathJoin(baseMediaDir, "part_1")
	if err = utils.PrepareDir(segmentDir); err != nil {
		return nil, err
	}

	var (
		segmentKey *m3u8.Key
		paramsList []DownloadParams
	)

	for index, segment := range playlist.Segments {
		if segment == nil {
			continue
		}

		var segmentUrl string

		if segmentUrl, err = utils.ResolveUrl(segment.URI, params.Url); err != nil {
			return nil, err
		}

		// 当前片段是否含有 key ，如果没有则使用上一个片段的 key
		if segment.Key != nil {
			segmentKey = segment.Key
			utils.ParseKeyFromUrl(segmentKey, params.Url)
		}

		var (
			method string
			key    []byte
			iv     []byte
		)

		if segmentKey == nil {
			method = ""
		} else {
			method = segmentKey.Method
			key = utils.CachedKey[segmentKey]
			if iv, err = hex.DecodeString(segmentKey.IV[2:]); err != nil {
				return nil, err
			}
		}

		paramsList = append(paramsList, DownloadParams{
			Name:  fmt.Sprintf("%06d.ts", index),
			Local: segmentDir,
			Url:   segmentUrl,
			Decoder: &Decoder{
				Method: method,
				Key:    key,
				Iv:     iv,
			},
		})
	}

	return paramsList, nil
}
