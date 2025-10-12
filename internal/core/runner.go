// Package core 包含命令执行器实现
package core

import (
	"bufio"
	"context"
	"os/exec"
)

// ExecRunner 基于 exec.CommandContext 的命令执行器
type ExecRunner struct{}

// NewExecRunner 创建命令执行器实例
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run 执行命令并逐行处理标准输出/错误输出
// ctx: 上下文控制（用于取消）
// binPath: 可执行文件路径
// args: 命令行参数列表
// onStdLine: 逐行回调函数
func (r *ExecRunner) Run(ctx context.Context, binPath string, args []string, onStdLine func(string)) error {
	// 使用 context 创建可取消的命令
	cmd := exec.CommandContext(ctx, binPath, args...)

	// 获取标准输出和标准错误管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return err
	}

	// 读取管道的函数
	readPipe := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			line := scanner.Text()
			if onStdLine != nil {
				onStdLine(line)
			}
		}
	}

	// 并发读取 stdout 和 stderr
	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)

	done := make(chan struct{}, 2)

	go func() {
		readPipe(stdoutScanner)
		done <- struct{}{}
	}()

	go func() {
		readPipe(stderrScanner)
		done <- struct{}{}
	}()

	// 等待两个 goroutine 完成
	<-done
	<-done

	// 等待命令执行完成
	return cmd.Wait()
}
