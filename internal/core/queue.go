// Package core 包含任务队列实现
package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// TaskQueue 任务队列，负责并发控制、任务调度与事件分发
type TaskQueue struct {
	downloader Downloader // 下载器实例
	maxRunner  int        // 最大并发数

	mu     sync.Mutex                   // 互斥锁
	queue  []DownloadParams             // 待执行任务队列
	active map[TaskID]context.CancelFunc // 活跃任务（任务ID -> 取消函数）
	proxy  string                       // 全局代理配置

	// 事件回调函数
	onStart    func(TaskID)
	onSuccess  func(TaskID)
	onFailed   func(TaskID, error)
	onStopped  func(TaskID)
	onProgress func(ProgressEvent)
	onMessage  func(MessageEvent)
}

// NewTaskQueue 创建任务队列实例
func NewTaskQueue(d Downloader, maxRunner int) *TaskQueue {
	return &TaskQueue{
		downloader: d,
		maxRunner:  maxRunner,
		active:     make(map[TaskID]context.CancelFunc),
	}
}

// SetProxy 设置全局代理
func (q *TaskQueue) SetProxy(p string) {
	q.mu.Lock()
	q.proxy = p
	q.mu.Unlock()
}

// SetMaxRunner 设置最大并发数
func (q *TaskQueue) SetMaxRunner(n int) {
	q.mu.Lock()
	q.maxRunner = n
	q.mu.Unlock()
	q.tryRun()
}

// Enqueue 添加任务到队列
func (q *TaskQueue) Enqueue(p DownloadParams) {
	q.mu.Lock()
	q.queue = append(q.queue, p)
	q.mu.Unlock()
	q.tryRun()
}

// Stop 停止指定任务
func (q *TaskQueue) Stop(id TaskID) error {
	q.mu.Lock()
	cancel, ok := q.active[id]
	q.mu.Unlock()

	if !ok {
		return ErrTaskNotFound
	}

	// 调用取消函数
	cancel()
	return nil
}

// tryRun 尝试运行队列中的任务（直到达到并发上限）
func (q *TaskQueue) tryRun() {
	for {
		q.mu.Lock()

		// 检查是否达到并发上限或队列为空
		if len(q.active) >= q.maxRunner || len(q.queue) == 0 {
			q.mu.Unlock()
			return
		}

		// 取出队列头部任务
		task := q.queue[0]
		q.queue = q.queue[1:]

		q.mu.Unlock()

		// 异步执行任务
		go q.execute(task)
	}
}

// execute 执行单个下载任务
func (q *TaskQueue) execute(p DownloadParams) {
	// 发送开始事件
	if q.onStart != nil {
		q.onStart(p.ID)
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 注册到活跃任务表
	q.mu.Lock()
	q.active[p.ID] = cancel
	proxy := q.proxy
	q.mu.Unlock()

	// 应用全局代理配置
	if proxy != "" {
		p.Proxy = proxy
	}

	// 执行下载
	err := q.downloader.Download(ctx, p, Callbacks{
		OnProgress: func(e ProgressEvent) {
			if q.onProgress != nil {
				q.onProgress(e)
			}
		},
		OnMessage: func(m MessageEvent) {
			if q.onMessage != nil {
				q.onMessage(m)
			}
		},
	})

	// 从活跃任务表中移除
	q.mu.Lock()
	delete(q.active, p.ID)
	q.mu.Unlock()

	// 根据错误类型发送相应事件
	switch {
	case err == nil:
		// 成功完成
		if q.onSuccess != nil {
			q.onSuccess(p.ID)
		}
	case errors.Is(err, context.Canceled):
		// 被取消
		if q.onStopped != nil {
			q.onStopped(p.ID)
		}
	default:
		// 失败
		if q.onFailed != nil {
			q.onFailed(p.ID, err)
		}
	}

	// 短暂延迟后尝试运行下一个任务
	time.AfterFunc(10*time.Millisecond, q.tryRun)
}

// 事件钩子注册方法（供 API 层使用）

func (q *TaskQueue) OnStart(fn func(TaskID)) {
	q.onStart = fn
}

func (q *TaskQueue) OnSuccess(fn func(TaskID)) {
	q.onSuccess = fn
}

func (q *TaskQueue) OnFailed(fn func(TaskID, error)) {
	q.onFailed = fn
}

func (q *TaskQueue) OnStopped(fn func(TaskID)) {
	q.onStopped = fn
}

func (q *TaskQueue) OnProgress(fn func(ProgressEvent)) {
	q.onProgress = fn
}

func (q *TaskQueue) OnMessage(fn func(MessageEvent)) {
	q.onMessage = fn
}
