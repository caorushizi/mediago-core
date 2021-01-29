package utils

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
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

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.132 Safari/537.36")

	if resp, err = client.Do(req); err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("响应状态码错误")
	}

	if content, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, err
	}

	return
}
