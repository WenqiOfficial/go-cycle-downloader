package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"

	"docker-cycler/pkg/docker"
	"docker-cycler/pkg/server"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	// 初始化状态和配置
	docker.InitState()

	// 设置嵌入的文件系统
	server.SetEmbeddedFS(templateFS)

	// 启动调度器
	server.StartScheduler()

	// 注册HTTP路由
	server.RegisterHandlers()

	// 启动服务器
	fmt.Println("服务已启动，访问控制台: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
