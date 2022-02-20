package engine

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"mediago/utils"
	"net/url"
	"os"
	"path"
)

func processSegment(params DownloadParams, errParams chan<- DownloadParams) {
	fmt.Printf("开始下载：%v\n", params)
	var (
		downloadFile *os.File
		filepath     = path.Join(params.Local, params.Name)
		content      []byte
		err          error
	)

	// 判断文件是否存在，如果存在则跳过下载
	if utils.FileExist(filepath) {
		return
	}

	if content, err = utils.HttpGet(params.Url); err != nil {
		utils.Logger.Error(err)
		errParams <- params
		fmt.Printf("下载失败，正在重试。")
		return
	}

	decoder := *params.Decoder
	switch decoder.Method {
	case "AES-128":
		if content, err = utils.AES128Decrypt(content, decoder.Key, decoder.Iv); err != nil {
			utils.Logger.Error(err)
			return
		}
	}

	if downloadFile, err = os.Create(filepath); err != nil {
		utils.Logger.Error(err)
		return
	}

	if _, err = downloadFile.Write(content); err != nil {
		utils.Logger.Error(err)
		return
	}

	if err = downloadFile.Close(); err != nil {
		utils.Logger.Error(err)
		return
	}
}

func processM3u8File(params DownloadParams) ([]DownloadParams, error) {
	var (
		err          error
		playlist     *m3u8.MediaPlaylist
		baseMediaDir string // 分片文件文件夹下载路径
		segmentDir   string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
		downloadList []DownloadParams
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
	fmt.Printf("下载完成123123123")
	m3u8Reader := bytes.NewReader(content)
	if p, listType, err = m3u8.DecodeFrom(m3u8Reader, true); err != nil {
		return nil, err
	}

	switch listType {
	case m3u8.MEDIA:
		playlist = p.(*m3u8.MediaPlaylist)

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
		)

		for index, segment := range playlist.Segments {
			if segment == nil {
				continue
			}

			var segmentUrl string

			if segmentUrl, err = utils.ResolveUrl(segment.URI, params.Url); err != nil {
				// todo: 处理错误
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

			downloadList = append(downloadList, DownloadParams{
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
		return downloadList, nil
	case m3u8.MASTER:
		masterPlaylist := p.(*m3u8.MasterPlaylist)
		variants := masterPlaylist.Variants

		// todo: 这里选择清晰度
		selected := variants[0]

		urlStr := selected.URI
		var urlOb *url.URL
		if !utils.IsUrl(urlStr) {
			if urlOb, err = url.Parse(params.Url); err != nil {
				return nil, err
			}
			urlOb.Path = urlStr
		}
		params.Url = urlOb.String()
		return processM3u8File(params)
	}
	return nil, errors.New("没有匹配到正确的媒体类型")
}
