package scheduler

import (
	"sync"
	"time"

	"mediago/utils"
)

type Scheduler struct {
	sync.WaitGroup
	Chs     chan int // 默认下载量
	Ans     chan int // 每个进程的下载状态
	Success int
}

// 创建新的调度器
func New(count int) (scheduler Scheduler) {
	return Scheduler{
		Chs: make(chan int, count),
		Ans: make(chan int),
	}
}

type Processor func() error

func (s *Scheduler) Work(id int, processor Processor) {
	var err error

	s.Chs <- id
	s.Add(1)

	if err = processor(); err != nil {
		utils.Logger.Errorf("任务 #%d 下载失败，3秒后进行重试: %s", id, err)
		time.Sleep(3 * time.Second)
		s.Work(id, processor)
	}

	filename := <-s.Chs
	s.Ans <- filename // 告知下载完成
	s.Done()
}
