package engine

import (
	"fmt"
	"mediago/scheduler"
	"mediago/utils"
)

var (
	s          scheduler.Scheduler
	paramsList []DownloadParams
)

func (e *Engine) Run(params DownloadParams) (err error) {

	// 开始初始化下载器
	utils.Logger.Debugf("初始化下载器")
	s = scheduler.New(15)

	if paramsList, err = processM3u8File(params); err != nil {
		return err
	}

	// 分发的下载线程
	go dispatchTask(paramsList)

	// 静静的等待每个下载完成
	for filename := range s.Ans {
		s.Success++
		utils.Logger.Infof("%06d: %s 片段已完成", s.Success, filename)
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

func dispatchTask(paramsList []DownloadParams) {
	utils.Logger.Infof("总共%d条任务", len(paramsList))
	s.Add(len(paramsList))

	for _, params := range paramsList {
		go func(params DownloadParams) {
			s.Work(params.Name, func() (err error) {
				err = processSegment(params)
				return
			})
		}(params)
	}

	s.Wait()
	close(s.Ans)
}
