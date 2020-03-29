package utils

import "testing"

func TestOutputMp4(t *testing.T) {
	if err := OutputMp4(
		"/Users/caorushizi/Downloads/test/鹡鸰女神1/filelist.txt",
		"/Users/caorushizi/Downloads/test/鹡鸰女神1.mp4",
	); err != nil {
		t.Error(err)
	}
}
