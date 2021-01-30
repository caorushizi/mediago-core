package engine

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"mediago/parser"
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

	if content, err = params.Download(params.Url); err != nil {
		return err
	}

	if content, err = params.Decoder.Decode(content); err != nil {
		return err
	}

	if downloadFile, err = os.Create(filepath); err != nil {
		return err
	}

	if _, err = downloadFile.Write(content); err != nil {
		return err
	}
	defer downloadFile.Close()

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

	if playlist, err = parser.ParseM3u8File(params.Url); err != nil {
		return nil, err
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
			Decoder: &SegmentDecoder{
				Method: method,
				Key:    key,
				Iv:     iv,
			},
			Download: parser.DownloadSegment,
		})
	}

	return paramsList, nil
}
