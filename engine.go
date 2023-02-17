package main

import (
	"fmt"
	"mediago/utils"
	"sync"
)

func createSegmentWorker() (chan<- DownloadParams, <-chan DownloadParams) {
	out := make(chan DownloadParams, 20)
	err := make(chan DownloadParams, 20)
	chs := make(chan DownloadParams, 15)
	wg := sync.WaitGroup{}

	go func() {
		for params := range out {
			wg.Add(1)
			chs <- params
			go func(params DownloadParams) {
				processSegment(params, err)
				<-chs
				wg.Done()
			}(params)
		}

		wg.Wait()
	}()

	return out, err
}

func (e *Engine) Run(params DownloadParams) (err error) {

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")

	var downloadList []DownloadParams
	if downloadList, err = processM3u8File(params); err != nil {
		return err
	}

	segmentChan, errChan := createSegmentWorker()
	var errQueue []DownloadParams
	retryTime := make(map[string]int)

	for {
		var activeWorkDownload DownloadParams
		var errWorkDownload DownloadParams
		var activeWorker chan<- DownloadParams
		var isErrQueue bool

		if len(downloadList) > 0 {
			activeWorkDownload = downloadList[0]
			activeWorker = segmentChan
		}

		if len(errQueue) > 0 && activeWorker == nil {
			errWorkDownload = errQueue[0]
			if value, exist := retryTime[errWorkDownload.Url]; !exist || value >= 15 {
				continue
			}
			retryTime[errWorkDownload.Url]++
			activeWorker = segmentChan
			isErrQueue = true
		}

		if len(downloadList) == 0 && len(errQueue) == 0 {
			fmt.Printf("下载完成\n")
			break
		}

		select {
		case segmentDownload := <-errChan:
			errQueue = append(errQueue, segmentDownload)
		case activeWorker <- activeWorkDownload:
			if !isErrQueue {
				downloadList = downloadList[1:]
			} else {
				errQueue = errQueue[1:]
			}
		}

	}

	if err = utils.ConcatVideo(params.Local, params.Name, "part_1"); err != nil {
		return fmt.Errorf("合并文件出错：%s", err)
	}

	utils.Logger.Infof("开始清理视频片段文件夹")
	segmentDir := utils.PathJoin(params.Local, params.Name)
	if err = utils.RemoveDir(segmentDir); err != nil {
		return
	}

	utils.Logger.Infof("下载完成")
	return
}
