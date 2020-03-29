package m3u8

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"mediago/utils"
	"net/url"
	"path"
	"regexp"
	"time"
)

type Segment struct {
	Duration time.Duration
	Url      url.URL
}

type ExtM3u8 struct {
	IsMaster   bool
	Name       string
	ProgramId  int
	BandWidth  int
	Resolution int
	Url        *url.URL
	Parts      []*ExtM3u8
	Segments   []Segment
}

func New(name string, urlString string) (playlist *ExtM3u8, err error) {
	playlist = &ExtM3u8{Name: name}
	// 检查 url 是否正确
	if playlist.Url, err = url.Parse(urlString); err != nil {
		return
	}

	return
}

func (m3u *ExtM3u8) Parse() (err error) {
	// 检查可用性
	if m3u.Url == nil {
		return errors.New("没有初始化 Url ~")
	}
	if m3u.Name == "" {
		return errors.New("没有初始化 Name ~")
	}

	if err = parse(m3u); err != nil {
		return
	}

	return
}

func parse(part *ExtM3u8) (err error) {
	// 开始处理 http 请求
	var repReader io.ReadCloser
	if repReader, err = utils.HttpClient(part.Url.String()); err != nil {
		return
	}
	defer repReader.Close()

	// 文件扫描
	var (
		fileScanner *bufio.Scanner
		fileLine    string
	)
	fileScanner = bufio.NewScanner(repReader)

	// 解析第一行必须是 `#EXTM3U`
	log.Println("开始扫描 m3u8 文件。")
	fileScanner.Scan()
	fileLine = fileScanner.Text()
	if fileLine != "#EXTM3U" {
		err = errors.New("不是一个 m3u8 文件")
		return
	}

	var (
		extInfReg     = regexp.MustCompile("^#EXTINF")
		extXStreamInf = regexp.MustCompile("^#EXT-X-STREAM-INF")
		tagReg        = regexp.MustCompile("^#EXT")
		commentsReg   = regexp.MustCompile("^#[^EXT]")
	)

	// 开始扫描 m3u8 文件
	var index int
	for fileScanner.Scan() {
		fileLine = fileScanner.Text()
		switch {
		// 注释直接跳过
		case commentsReg.MatchString(fileLine):

		/* START 考虑到的标签 */
		case extXStreamInf.MatchString(fileLine):
			part.IsMaster = true
		case extInfReg.MatchString(fileLine):
		/* END 考虑到的标签 */

		// 没有考虑到的 Tag 标签
		case tagReg.MatchString(fileLine):
		default:
			// m3u8 文件属性列表已经扫描完成，下面的文字都是包含片段信息的文本
			// 或者包含片段内容，一般是 url 的绝对路径或者相对路径

			// 首先解析地址，然后判断类型
			var segmentUrl url.URL
			if segmentUrl, err = part.handleUrlParse(fileLine); err != nil {
				err = fmt.Errorf("解析url片段出错：%v", err)
				continue
			}

			if part.IsMaster {
				log.Println("发现 master 列表。")
				index++
				// 是主列表
				var (
					newPart *ExtM3u8
				)
				name := fmt.Sprintf("part_%d", index)
				if newPart, err = New(name, segmentUrl.String()); err != nil {
					return
				}
				part.Parts = append(part.Parts, newPart)
				log.Println("开始解析 playlist 列表。")
				if err = parse(newPart); err != nil {
					return
				}
			} else {
				// 是播放列表
				segment := Segment{Url: segmentUrl}
				part.Segments = append(part.Segments, segment)
			}

		}
	}

	// 没有错误直接返回
	return
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
