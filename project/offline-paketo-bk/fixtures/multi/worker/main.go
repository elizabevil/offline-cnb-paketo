package main

import (
	"fmt"
	"time"
)

// 后台进程：不监听端口，用于验证多模块各自产出层镜像
func main() {
	for {
		fmt.Println("worker tick")
		time.Sleep(5 * time.Second)
	}
}
