package engine

import (
	"mediago/utils"
)

type Decoder interface {
	Decode(content []byte) ([]byte, error)
}

type SegmentDecoder struct {
	Method string
	Key    []byte
	Iv     []byte
}

func (d *SegmentDecoder) Decode(content []byte) ([]byte, error) {
	var err error

	switch d.Method {
	case "AES-128":
		if content, err = utils.AES128Decrypt(content, d.Key, d.Iv); err != nil {
			return nil, err
		}
	}

	return content, nil
}

type M3u8Decoder struct {
}

func (m *M3u8Decoder) Decode(content []byte) ([]byte, error) {
	panic("implement me")
}

type Engine struct {
}

type DownloadFn func(url string) ([]byte, error)

type DownloadParams struct {
	Name     string
	Local    string
	Url      string
	Decoder  Decoder
	Download DownloadFn
}
