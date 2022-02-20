package engine

type Decoder struct {
	Method string
	Key    []byte
	Iv     []byte
}

type Engine struct {
}

type DownloadParams struct {
	Name    string
	Local   string
	Url     string
	Decoder *Decoder
}

type M3u8DownloadParams struct {
}
