// Package core 控制台输出解析与进度节流逻辑
package core

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// parseState 解析状态
type parseState struct {
	ready   bool    // 是否已进入 ready 状态
	percent float64 // 当前进度百分比
	speed   string  // 当前下载速度
	isLive  bool    // 是否为直播
}

// lineParser 控制台输出解析器
type lineParser struct {
	percentReg *regexp.Regexp
	speedReg   *regexp.Regexp
	errorReg   *regexp.Regexp
	startReg   *regexp.Regexp
	isLiveReg  *regexp.Regexp
}

// newLineParser 创建解析器
func newLineParser(cr ConsoleReg) (*lineParser, error) {
	lp := &lineParser{}
	var err error

	if cr.Percent != "" {
		lp.percentReg, err = regexp.Compile(cr.Percent)
		if err != nil {
			return nil, err
		}
	}
	if cr.Speed != "" {
		lp.speedReg, err = regexp.Compile(cr.Speed)
		if err != nil {
			return nil, err
		}
	}
	if cr.Error != "" {
		lp.errorReg, err = regexp.Compile(cr.Error)
		if err != nil {
			return nil, err
		}
	}
	if cr.Start != "" {
		lp.startReg, err = regexp.Compile(cr.Start)
		if err != nil {
			return nil, err
		}
	}
	if cr.IsLive != "" {
		lp.isLiveReg, err = regexp.Compile(cr.IsLive)
		if err != nil {
			return nil, err
		}
	}

	return lp, nil
}

// parse 解析一行控制台输出，返回事件类型和错误信息
func (lp *lineParser) parse(line string, state *parseState) (event string, errMsg string) {
	// 错误行
	if lp.errorReg != nil && lp.errorReg.MatchString(line) {
		return "", line
	}

	// 是否直播
	if lp.isLiveReg != nil && lp.isLiveReg.MatchString(line) {
		state.isLive = true
	}

	// 检测开始标识，进入 ready 状态
	if !state.ready && lp.startReg != nil && lp.startReg.MatchString(line) {
		return "ready", ""
	}

	// 解析进度百分比（记录是否匹配到）
	matchedPercent := false
	if lp.percentReg != nil {
		matches := lp.percentReg.FindStringSubmatch(line)
		if len(matches) > 1 {
			if percent, err := strconv.ParseFloat(matches[1], 64); err == nil {
				state.percent = percent
				matchedPercent = true
			}
		}
	}

	// 解析下载速度（记录是否匹配到）
	matchedSpeed := false
	if lp.speedReg != nil {
		matches := lp.speedReg.FindStringSubmatch(line)
		if len(matches) > 1 {
			state.speed = strings.TrimSpace(matches[1])
			matchedSpeed = true
		}
	}

	// 若未 ready，但已解析到进度或速度，自动进入 ready（即便配置了 start 但未命中）
	if !state.ready && (matchedPercent || matchedSpeed) {
		state.ready = true
		return "ready", ""
	}

	return "", ""
}

// progressRecord 进度记录
type progressRecord struct {
	lastPercent float64
	lastSpeed   string
	lastUpdate  time.Time
}

// progressTracker 进度节流：200ms + （0.5% 或 速度变化）
type progressTracker struct {
	mu      sync.Mutex
	records map[TaskID]*progressRecord
}

// newTracker 创建进度追踪器
func newTracker() *progressTracker {
	return &progressTracker{
		records: make(map[TaskID]*progressRecord),
	}
}

// shouldUpdate 判断是否应当上报进度
// 策略：200ms 节流窗口内只允许一次；窗口外若进度>=0.5% 或 速度变化则上报
func (pt *progressTracker) shouldUpdate(id TaskID, percent float64, speed string) bool {
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

// update 更新进度记录
func (pt *progressTracker) update(id TaskID, percent float64, speed string) {
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

// remove 移除某任务的进度记录
func (pt *progressTracker) remove(id TaskID) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.records, id)
}
