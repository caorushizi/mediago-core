package utils

import (
	"path"
	"strings"
)

func PathJoin(elem ...string) string {
	return path.Join(elem...)
}

func NormalizePath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}

	if !strings.HasPrefix(dir, "./") {
		return "./" + dir
	}

	return dir
}
