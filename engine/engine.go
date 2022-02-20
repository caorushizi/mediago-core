package engine

import (
	"fmt"
	"mediago/utils"
)

func createM3u8FileWorker(params DownloadParams) (<-chan DownloadParams, <-chan DownloadParams) {
	out := make(chan DownloadParams, 10)
	done := make(chan DownloadParams)

	go processM3u8File(params, out)

	return out, done
}

func createSegmentWorker() (chan<- DownloadParams, <-chan DownloadParams) {
	out := make(chan DownloadParams, 10)
	err := make(chan DownloadParams, 20)

	go processSegment(out, err)

	return out, err
}

func (e *Engine) Run(params DownloadParams) (err error) {

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")

	out, done := createM3u8FileWorker(params)
	segmentChan, errChan := createSegmentWorker()
	var errQueue []DownloadParams
	var workQueue []DownloadParams
	var parseDone bool
	retryTime := make(map[string]int)

	for {
		var activeWorkDownload DownloadParams
		var errWorkDownload DownloadParams
		var activeWorker chan<- DownloadParams
		var isErrQueue bool

		if len(workQueue) > 0 {
			activeWorkDownload = workQueue[0]
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

		if parseDone && len(workQueue) == 0 && len(errQueue) == 0 {
			fmt.Printf("下载完成\n")
			break
		}

		select {
		case segmentDownload := <-out:
			workQueue = append(workQueue, segmentDownload)
		case segmentDownload := <-errChan:
			errQueue = append(errQueue, segmentDownload)
		case activeWorker <- activeWorkDownload:
			if !isErrQueue {
				workQueue = workQueue[1:]
			} else {
				errQueue = errQueue[1:]
			}
		case <-done:
			parseDone = true
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
