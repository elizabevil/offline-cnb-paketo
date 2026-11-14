package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte("ok"))
	})
	// 启动服务，监听 0.0.0.0:8080
	fmt.Println("server run on http://127.0.0.1:80")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		panic(err)
	}
}
