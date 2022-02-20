package engine

import (
	"fmt"
	"mediago/utils"
)

func createM3u8FileWorker(params DownloadParams) <-chan DownloadParams {
	out := make(chan DownloadParams, 10)

	go processM3u8File(params, out)

	return out
}

func (e *Engine) Run(params DownloadParams) (err error) {

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")

	out := createM3u8FileWorker(params)
	var errQueue []DownloadParams
	var workQueue []DownloadParams
	retryTime := make(map[string]int)

	var (
		activeWork DownloadParams
		activeErr  DownloadParams
	)
	for {
		activeWorker := make(chan<- DownloadParams)
		errWorker := make(chan<- DownloadParams)
		if len(workQueue) > 0 {
			activeWork = workQueue[0]
		}
		if len(errQueue) > 0 {
			activeErr = errQueue[0]
		}

		select {
		case segmentDownload := <-out:
			fmt.Printf("%v\n", segmentDownload)
			workQueue = append(workQueue, segmentDownload)
		case activeWorker <- activeWork:
			fmt.Printf("activeWork %v\n", activeWorker)
			if err := processSegment(activeWork); err != nil {
				utils.Logger.Error(err)
				errQueue = append(errQueue, activeWork)
				retryTime[activeWork.Url]++
			} else {
				workQueue = workQueue[1:]
			}
		case errWorker <- activeErr:
			fmt.Printf("activeErr %v\n", activeErr)
			value, exist := retryTime[activeErr.Url]
			if !exist {
				break
			}
			if value >= 15 {
				break
			}
			if err := processSegment(activeErr); err != nil {
				utils.Logger.Error(err)
				errQueue = append(errQueue, activeErr)
				retryTime[activeErr.Url]++
			} else {
				errQueue = errQueue[1:]
			}
		default:
		}

	}

	//// 分发的下载线程
	//go func(paramsList []DownloadParams) {
	//	utils.Logger.Infof("总共%d条任务", len(paramsList))
	//	s.Add(len(paramsList))
	//
	//	for _, params := range paramsList {
	//		go func(params DownloadParams) {
	//			s.Work(params.Name, func() (err error) {
	//				err = processSegment(params)
	//				return
	//			})
	//		}(params)
	//	}
	//
	//	s.Wait()
	//	close(s.Ans)
	//}(paramsList)
	//
	//// 静静的等待每个下载完成
	//for filename := range s.Ans {
	//	s.Success++
	//	utils.Logger.Infof("%06d: %s 片段已完成", s.Success, filename)
	//}

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
