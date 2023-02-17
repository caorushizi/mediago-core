package utils

import (
	"net/url"
	"path"
)

func ResolveUrl(segmentText string, baseUrl string) (urlString string, err error) {
	// 如果 m3u8 列表中已经是完整的 url
	if IsUrl(segmentText) {
		return segmentText, nil
	}

	var resultUrl *url.URL
	if resultUrl, err = url.Parse(baseUrl); err != nil {
		return "", err
	}
	if path.IsAbs(segmentText) {
		resultUrl.Path = segmentText
	} else {
		tempBaseUrl := path.Dir(resultUrl.Path)
		resultUrl.Path = path.Join(tempBaseUrl, segmentText)
	}

	return resultUrl.String(), nil
}
