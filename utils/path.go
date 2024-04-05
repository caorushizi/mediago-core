package utils

import (
	"path"
)

func PathJoin(elem ...string) string {
	return path.Join(elem...)
}

func NormalizePath(dir string) string {
	if dir == "" {
		return "./"
	}

	return dir
}
