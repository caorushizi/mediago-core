package utils

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"

	"caorushizi.cn/mediago/config"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type M3u8Downloader struct {
	cmd      *exec.Cmd
	url      string
	progress float32
	speed    float32
	duration int
	error
}

const binPath = "D:\\Workspace\\Github\\mediago\\bin\\N_m3u8DL-CLI.exe"

func NewM3u8Downloader(url string, name string) *M3u8Downloader {
	cmd := exec.Command(binPath, url, "--workDir", config.Config.Local, "--saveName", name)

	return &M3u8Downloader{
		url: url,
		cmd: cmd,
	}
}

func (d *M3u8Downloader) Start() error {
	// TODO: std error
	stdout, _ := d.cmd.StdoutPipe()

	go func() {
		reader := transform.NewReader(stdout, simplifiedchinese.GBK.NewDecoder())
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			text := scanner.Text()
			d.parse(text)
		}
	}()

	if err := d.cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := d.cmd.Wait(); err != nil {
		}
	}()

	return nil
}

var (
	startRegexp    = regexp.MustCompile(`开始下载文件`)
	progressRegexp = regexp.MustCompile(`Progress: (\d+)/(\d+) .* \(([\d.]+) MB/s`)
	completeRegexp = regexp.MustCompile(`已下载完毕`)
	combineRegexp  = regexp.MustCompile(`开始合并分片...`)
	doneRegexp     = regexp.MustCompile(`任务结束`)
)

func (d *M3u8Downloader) parse(text string) error {
	switch {
	case startRegexp.MatchString(text):
		fmt.Println("startRegexp")
	case progressRegexp.MatchString(text):
		match := progressRegexp.FindStringSubmatch(text)
		fmt.Printf("Current Download: %s, Total: %s, Speed: %s MB/s\n",
			match[1], match[2], match[3])
	case completeRegexp.MatchString(text):
		fmt.Println("completeRegexp")
	case combineRegexp.MatchString(text):
		fmt.Println("combineRegexp")
	case doneRegexp.MatchString(text):
		fmt.Println("doneRegexp")
	default:
		fmt.Println(text)
	}

	return nil
}
