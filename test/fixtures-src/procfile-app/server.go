package main

import (
	"fmt"
	"net/http"
	"os"
)

// 最小静态服务：procfile fixture 用，编译后不依赖任何运行时 buildpack
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok from procfile fixture")
	})
	_ = http.ListenAndServe(":"+port, nil)
}
