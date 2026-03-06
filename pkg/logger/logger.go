package logger

import (
	"log"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

// Info 基础信息日志
func Info(category, msg string) {
	log.Printf("%s[INFO][%s]%s %s", colorBlue, category, colorReset, msg)
}

// Error 错误日志
func Error(category string, err error, msg string) {
	log.Printf("%s[ERROR][%s]%s %s:%v", colorRed, category, colorReset, msg, err)
}

// Boot 启动日志
func Boot(msg string) {
	log.Printf("%s[BOOT]%s %s", colorGreen, colorReset, msg)
}

// Cache 缓存日志
func Cache(hitType string, msg string) {
	log.Printf("%s[CACHE][%s]%s %s", colorYellow, hitType, colorReset, msg)
}

func Access(method, path string, status int, duration time.Duration, cacheHit string) {
	log.Printf("%s[ACCESS]%s %s %s | %d | %v | CacheHit:%s",
		colorCyan, colorReset, method, path, status, duration, cacheHit)
}
