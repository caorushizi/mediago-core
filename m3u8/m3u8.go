package m3u8

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"mediago/utils"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type ExtM3u8 struct {
	Name     string
	Url      *url.URL
	Segments []url.URL
}

func New(name string, urlString string) (playlist ExtM3u8, err error) {
	playlist.Name = name
	// 检查 url 是否正确
	if playlist.Url, err = url.Parse(urlString); err != nil {
		return
	}

	return
}

func (m3u *ExtM3u8) Parse() (err error) {
	// 开始处理 http 请求
	var repReader io.ReadCloser
	if repReader, err = utils.HttpClient(m3u.Url.String()); err != nil {
		return
	}
	defer repReader.Close()

	// 文件扫描
	var (
		fileScanner *bufio.Scanner
		segments    []url.URL
	)
	fileScanner = bufio.NewScanner(repReader)

	// 解析第一行必须是 `#EXTM3U`
	fileScanner.Scan()
	text := fileScanner.Text()
	if text != "#EXTM3U" {
		err = errors.New("不是一个 m3u8 文件")
		return
	}

	var (
		extInfReg   = regexp.MustCompile("^EXTINF")
		commentsReg = regexp.MustCompile("^#[^EXT]")
	)

	for fileScanner.Scan() {
		text := fileScanner.Text()
		switch {
		case extInfReg.MatchString(text):
		case strings.HasPrefix(text, "#EXT"):
		case commentsReg.MatchString(text):
			// 这一行是注释直接跳过
		default:
			// 拼接与 url
			var segmentUrl url.URL

			if segmentUrl, err = m3u.handleUrlParse(text); err != nil {
				err = fmt.Errorf("解析url片段出错：%v", err)
				continue
			}
			segments = append(segments, segmentUrl)
		}
	}

	m3u.Segments = segments
	return
}

func parseTag() {

}

func parseAttr() {

}

func (m3u *ExtM3u8) handleUrlParse(segmentText string) (segmentUrl url.URL, err error) {
	var (
		pSegmentUrl *url.URL
		tempBaseUrl string
	)

	// 如果 m3u8 列表中已经是完整的 url
	if utils.IsUrl(segmentText) {
		// 解析 url
		if pSegmentUrl, err = url.Parse(segmentText); err != nil {
			return
		}
		return *pSegmentUrl, nil
	}

	// 不是完整的 url
	segmentUrl = *m3u.Url

	if path.IsAbs(segmentText) {
		segmentUrl.Path = segmentText
	} else {
		tempBaseUrl = path.Dir(segmentUrl.Path)
		segmentUrl.Path = path.Join(tempBaseUrl, segmentText)
	}

	return
}
