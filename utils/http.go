package utils

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

func HttpGet(url string) (content []byte, err error) {
	// 初始化变量
	var (
		client    *http.Client
		req       *http.Request
		resp      *http.Response
		transport *http.Transport
	)

	transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// 创建 http client
	client = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return
	}

	// TODO: 实现 http 方法
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.132 Safari/537.36")
	for _, header := range strings.Split(Headers, "|") {
		if header == "" {
			continue
		}
		temp := strings.Split(header, "~")
		req.Header.Add(temp[0], temp[1])
	}

	if resp, err = client.Do(req); err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("响应状态码错误：%d", resp.StatusCode)
		return nil, errors.New(errMsg)
	}

	if content, err = httputil.DumpResponse(resp, true); err != nil {
		return nil, err
	}

	return
}
