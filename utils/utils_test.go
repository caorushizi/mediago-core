package utils

import "testing"

func TestOutputMp4(t *testing.T) {

	OutputMp4("/Users/caorushizi/Downloads/test/猎心者26/fileList.txt", "/Users/caorushizi/Downloads/test/猎心者26.mp4")
}

func TestLogger(t *testing.T) {
	Logger()
}
