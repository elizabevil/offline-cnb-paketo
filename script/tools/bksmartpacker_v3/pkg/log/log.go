package log

import (
	"fmt"
	"log"
	"sync"
)

var lock = new(sync.Mutex)

// Info 日志
func Info(v ...any) {
	lock.Lock()
	log.Print("##[info]")
	log.Println(v...)
	lock.Unlock()
}

// Warn 日志
func Warn(v ...any) {
	lock.Lock()
	log.Print("##[warning]")
	log.Println(v...)
	lock.Unlock()
}

// Error 日志
func Error(v ...any) {
	lock.Lock()
	log.Print("##[error]")
	log.Println(v...)
	lock.Unlock()
}

// Debug 日志
func Debug(v ...any) {
	lock.Lock()
	log.Print("##[debug]")
	log.Println(v...)
	lock.Unlock()
}

// Command 日志
func Command(v ...any) {
	lock.Lock()
	log.Print("##[command]")
	log.Println(v...)
	lock.Unlock()
}

// Group 分组日志开始
func Group(v ...any) {
	lock.Lock()
	fmt.Print("##[group]")
	fmt.Println(v...)
	lock.Unlock()
}

// EndGroup 分组日志结束
func EndGroup(v ...any) {
	lock.Lock()
	log.Print("##[endgroup]")
	log.Println(v...)
	lock.Unlock()
}
