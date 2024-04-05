package utils

import (
	"errors"
	"os"
)

func mkdir(dir string) (err error) {
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}
	return
}

func FileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func PrepareDir(dir string) error {
	// 检查下载路径是否存在
	// 并且检查时候有权限写入文件
	var (
		fileInfo os.FileInfo
		err      error
	)

	if fileInfo, err = os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			if err = mkdir(dir); err != nil {
				return err
			}
			return nil
		}

		if os.IsPermission(err) {
			return err
		}

		return err
	}

	if !fileInfo.IsDir() {
		return errors.New("已经存在同名的文件")
	}

	return nil
}

func RemoveDir(dirname string) (err error) {
	//Logger.Infof("开始删除文件夹：%s", dirname)
	if err = os.RemoveAll(dirname); err != nil {
		return
	}
	return
}
