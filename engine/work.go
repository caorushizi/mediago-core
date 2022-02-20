package engine

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/grafov/m3u8"
	"mediago/utils"
	"net/url"
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
		utils.Logger.Error(err)
		return err
	}

	if err = downloadFile.Close(); err != nil {
		return err
	}

	return nil
}

func processM3u8File(params DownloadParams, out chan DownloadParams) {
	var (
		err          error
		playlist     *m3u8.MediaPlaylist
		baseMediaDir string // 分片文件文件夹下载路径
		segmentDir   string // 分片文件下载具体路径 =  baseMediaDir + segmentDirName
	)

	outFile := utils.PathJoin(params.Local, fmt.Sprintf("%s.mp4", params.Name))
	if utils.FileExist(outFile) {
		utils.Logger.Infof("文件已经存在！")
		return
	}

	if err = utils.PrepareDir(params.Local); err != nil {
		utils.Logger.Error(err)
		return
	}

	// 开始处理 http 请求
	utils.Logger.Debugf("开始解析 m3u8 文件")
	var (
		listType m3u8.ListType
		p        m3u8.Playlist
	)

	var content []byte
	if content, err = utils.HttpGet(params.Url); err != nil {
		utils.Logger.Error(err)
		return
	}
	m3u8Reader := bytes.NewReader(content)
	if p, listType, err = m3u8.DecodeFrom(m3u8Reader, true); err != nil {
		utils.Logger.Error(err)
		return
	}

	switch listType {
	case m3u8.MEDIA:
		playlist = p.(*m3u8.MediaPlaylist)
		fmt.Printf("%v\n", playlist)

		if playlist == nil {
			utils.Logger.Infof("片段列表为空")
			return
		}

		// 创建视频集合文件夹
		baseMediaDir = utils.PathJoin(params.Local, params.Name)
		if err = utils.PrepareDir(baseMediaDir); err != nil {
			utils.Logger.Error(err)
			return
		}

		// 创建视频片段文件夹
		segmentDir = utils.PathJoin(baseMediaDir, "part_1")
		if err = utils.PrepareDir(segmentDir); err != nil {
			utils.Logger.Error(err)
			return
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
				utils.Logger.Error(err)
				return
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
					utils.Logger.Error(err)
					return
				}
			}

			params := DownloadParams{
				Name:  fmt.Sprintf("%06d.ts", index),
				Local: segmentDir,
				Url:   segmentUrl,
				Decoder: &Decoder{
					Method: method,
					Key:    key,
					Iv:     iv,
				},
			}

			out <- params
		}
	case m3u8.MASTER:
		masterPlaylist := p.(*m3u8.MasterPlaylist)

		fmt.Printf("%v\n", masterPlaylist)
		variants := masterPlaylist.Variants
		fmt.Printf("%v\n", variants)

		// todo: 这里选择清晰度
		selected := variants[0]

		urlStr := selected.URI
		var urlOb *url.URL
		if !utils.IsUrl(urlStr) {
			if urlOb, err = url.Parse(params.Url); err != nil {
				utils.Logger.Error(err)
				return
			}
			urlOb.Path = urlStr
		}
		params.Url = urlOb.String()
		processM3u8File(params, out)
		return
	}

}
