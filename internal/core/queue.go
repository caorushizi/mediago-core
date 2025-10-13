// Package core 包含任务队列实现
package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"caorushizi.cn/mediago/internal/logger"
	"go.uber.org/zap"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// TaskQueue 任务队列，负责并发控制、任务调度与事件分发
type TaskQueue struct {
	downloader Downloader // 下载器实例
	maxRunner  int        // 最大并发数

	mu     sync.RWMutex                 // 读写锁
	queue  []DownloadParams             // 待执行任务队列
	active map[TaskID]context.CancelFunc // 活跃任务（任务ID -> 取消函数）
	tasks  map[TaskID]*TaskInfo         // 任务信息表（任务ID -> 任务信息）
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
		tasks:      make(map[TaskID]*TaskInfo),
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
	// 初始化任务信息
	q.tasks[p.ID] = &TaskInfo{
		ID:      p.ID,
		Type:    p.Type,
		URL:     p.URL,
		Name:    p.Name,
		Status:  StatusPending,
		Percent: 0,
		Speed:   "",
		IsLive:  false,
	}
	queueLen := len(q.queue)
	q.mu.Unlock()

	logger.Info("Task enqueued",
		zap.Int64("id", int64(p.ID)),
		zap.String("type", string(p.Type)),
		zap.String("name", p.Name),
		zap.Int("queueLength", queueLen))

	q.tryRun()
}

// Stop 停止指定任务
func (q *TaskQueue) Stop(id TaskID) error {
	q.mu.Lock()
	cancel, ok := q.active[id]
	q.mu.Unlock()

	if !ok {
		logger.Warn("Attempted to stop non-existent task", zap.Int64("id", int64(id)))
		return ErrTaskNotFound
	}

	logger.Info("Stopping task", zap.Int64("id", int64(id)))
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
	logger.Info("Executing task",
		zap.Int64("id", int64(p.ID)),
		zap.String("type", string(p.Type)))

	// 更新任务状态为下载中
	q.mu.Lock()
	if task, ok := q.tasks[p.ID]; ok {
		task.Status = StatusDownloading
	}
	q.mu.Unlock()

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
	activeCount := len(q.active)
	q.mu.Unlock()

	logger.Debug("Task activated",
		zap.Int64("id", int64(p.ID)),
		zap.Int("activeCount", activeCount))

	// 应用全局代理配置
	if proxy != "" {
		p.Proxy = proxy
		logger.Debug("Applied global proxy to task",
			zap.Int64("id", int64(p.ID)),
			zap.String("proxy", proxy))
	}

	// 执行下载
	err := q.downloader.Download(ctx, p, Callbacks{
		OnProgress: func(e ProgressEvent) {
			// 更新任务进度信息
			q.mu.Lock()
			if task, ok := q.tasks[p.ID]; ok {
				task.Percent = e.Percent
				task.Speed = e.Speed
				task.IsLive = e.IsLive
			}
			q.mu.Unlock()

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
	activeCount = len(q.active)
	q.mu.Unlock()

	logger.Debug("Task deactivated",
		zap.Int64("id", int64(p.ID)),
		zap.Int("activeCount", activeCount))

	// 根据错误类型发送相应事件并更新任务状态
	switch {
	case err == nil:
		// 成功完成
		logger.Info("Task completed successfully", zap.Int64("id", int64(p.ID)))
		q.mu.Lock()
		if task, ok := q.tasks[p.ID]; ok {
			task.Status = StatusSuccess
			task.Percent = 100
		}
		q.mu.Unlock()
		if q.onSuccess != nil {
			q.onSuccess(p.ID)
		}
	case errors.Is(err, context.Canceled):
		// 被取消
		logger.Info("Task was stopped", zap.Int64("id", int64(p.ID)))
		q.mu.Lock()
		if task, ok := q.tasks[p.ID]; ok {
			task.Status = StatusStopped
		}
		q.mu.Unlock()
		if q.onStopped != nil {
			q.onStopped(p.ID)
		}
	default:
		// 失败
		logger.Error("Task failed",
			zap.Int64("id", int64(p.ID)),
			zap.Error(err))
		q.mu.Lock()
		if task, ok := q.tasks[p.ID]; ok {
			task.Status = StatusFailed
			task.Error = err.Error()
		}
		q.mu.Unlock()
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

// GetTask 获取指定任务的信息
func (q *TaskQueue) GetTask(id TaskID) (*TaskInfo, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	task, ok := q.tasks[id]
	if !ok {
		return nil, false
	}
	// 返回副本，避免外部修改
	taskCopy := *task
	return &taskCopy, true
}

// GetAllTasks 获取所有任务的信息
func (q *TaskQueue) GetAllTasks() []TaskInfo {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]TaskInfo, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, *task)
	}
	return tasks
}
