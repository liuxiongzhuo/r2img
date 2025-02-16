package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"r2img/internal/config" // 导入内部包
	"r2img/internal/server"
	"strconv"
)

func main() {
	err := os.MkdirAll("./i", 0755) // 创建目录，并设置权限
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return // 或者 panic(err)，根据你的错误处理策略
	}

	// 加载配置
	configPath := "config.json" // 配置文件路径
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err) // 使用 log.Fatalf 退出程序
	}

	// 创建 HTTP 服务器
	srv := server.NewServer(cfg) // 使用 internal/server 包中的 NewServer

	// 启动服务器
	log.Printf("服务启动，监听端口: %d\n", cfg.Port)
	err = http.ListenAndServe(":"+strconv.Itoa(cfg.Port), srv) // srv 现在实现了 http.Handler
	if err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
