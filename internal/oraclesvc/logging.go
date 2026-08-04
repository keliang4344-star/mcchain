package oraclesvc

// 生产加固：统一的分级结构化日志。
//
// 输出格式（单行、易被 journald / Loki / grep 解析）：
//
//	2026-08-04T02:49:25Z level=INFO comp=oracle msg="..."
//
// 仅依赖标准库 log/os/sync/time，不引入任何第三方日志库。

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// 日志级别。
const (
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// baseLogger 直接写 stderr；不使用 log 包自带的时间戳标志，
// 时间由本文件统一按 RFC3339（UTC）格式化，保证字段顺序稳定。
var (
	logMu      sync.Mutex
	baseLogger = log.New(os.Stderr, "", 0)
	logComp    = "oracle"
)

// SetLogComponent 设置日志中的 comp 字段（例如 "oracle-signer" / "oracle-attestor"），
// 便于同一部署下区分两个预言机进程的日志。
func SetLogComponent(name string) {
	if name == "" {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	logComp = name
}

// logf 按级别输出一行结构化日志。
func logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// 折叠换行，保证「一条日志一行」，避免多行日志破坏采集。
	msg = strings.ReplaceAll(strings.ReplaceAll(msg, "\r", " "), "\n", " ")

	logMu.Lock()
	comp := logComp
	logMu.Unlock()

	baseLogger.Printf("%s level=%s comp=%s msg=%q",
		time.Now().UTC().Format(time.RFC3339), level, comp, msg)
}

// Infof 记录一条 INFO 级别日志。
func Infof(format string, args ...interface{}) { logf(LevelInfo, format, args...) }

// Warnf 记录一条 WARN 级别日志（可疑但服务仍可继续）。
func Warnf(format string, args ...interface{}) { logf(LevelWarn, format, args...) }

// Errorf 记录一条 ERROR 级别日志（操作失败或服务已降级）。
func Errorf(format string, args ...interface{}) { logf(LevelError, format, args...) }
