// Package parser 进度追踪与节流
package parser

import (
	"math"
	"sync"
	"time"
)

// TaskID 任务唯一标识符
type TaskID string

// progressRecord 进度记录
type progressRecord struct {
	lastPercent float64
	lastSpeed   string
	lastUpdate  time.Time
}

// ProgressTracker 进度节流：200ms + （0.5% 或 速度变化）
type ProgressTracker struct {
	mu      sync.Mutex
	records map[TaskID]*progressRecord
}

// NewTracker 创建进度追踪器
func NewTracker() *ProgressTracker {
	return &ProgressTracker{
		records: make(map[TaskID]*progressRecord),
	}
}

// ShouldUpdate 判断是否应当上报进度
// 策略：200ms 节流窗口内只允许一次；窗口外若进度>=0.5% 或 速度变化则上报
func (pt *ProgressTracker) ShouldUpdate(id TaskID, percent float64, speed string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	rec, exists := pt.records[id]
	if !exists {
		// 首次上报
		return true
	}

	now := time.Now()
	timeDiff := now.Sub(rec.lastUpdate)

	// 时间间隔小于 200ms 不上报
	if timeDiff < 200*time.Millisecond {
		return false
	}

	// 优先：进度变化达到 0.5% 直接上报
	percentDiff := math.Abs(percent - rec.lastPercent)
	if percentDiff >= 0.5 {
		return true
	}

	// 其次：速度变化也允许上报（直播/未知总量场景）
	if speed != rec.lastSpeed {
		return true
	}

	return false
}

// Update 更新进度记录
func (pt *ProgressTracker) Update(id TaskID, percent float64, speed string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	rec, exists := pt.records[id]
	if !exists {
		pt.records[id] = &progressRecord{
			lastPercent: percent,
			lastSpeed:   speed,
			lastUpdate:  time.Now(),
		}
	} else {
		rec.lastPercent = percent
		rec.lastSpeed = speed
		rec.lastUpdate = time.Now()
	}
}

// Remove 移除某任务的进度记录
func (pt *ProgressTracker) Remove(id TaskID) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.records, id)
}
