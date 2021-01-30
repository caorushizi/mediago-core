package scheduler

import (
	"sync"
	"time"

	"mediago/utils"
)

type Scheduler struct {
	sync.WaitGroup
	chs     chan string // 默认下载量
	Ans     chan string // 每个进程的下载状态
	Success int
}

// 创建新的调度器
func New(count int) (scheduler Scheduler) {
	return Scheduler{
		chs: make(chan string, count),
		Ans: make(chan string),
	}
}

type Processor func() error

func (s *Scheduler) Work(id string, processor Processor) {
	var err error

	s.chs <- id

	if err = processor(); err != nil {
		utils.Logger.Errorf("任务 #%s 下载失败，3秒后进行重试: %s", id, err)
		time.Sleep(3 * time.Second)
		s.Work(id, processor)
	} else {
		filename := <-s.chs
		s.Ans <- filename // 告知下载完成
		s.Done()
	}
}
