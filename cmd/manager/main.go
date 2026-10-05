package main

import (
	"flag"
	"log"

	"merit/internal/manager"
)

func main() {
	addr := flag.String("addr", ":3000", "面板与 Agent 通信监听地址")
	data := flag.String("data", "data/merit.json", "数据文件路径")
	agentDir := flag.String("agent-dir", "dist", "存放 agent 二进制的目录（merit-agent-linux-amd64 等）")
	publicURL := flag.String("public-url", "", "对外访问地址，例如 http://1.2.3.4:3000，留空则自动使用请求 Host")
	flag.Parse()

	srv, err := manager.New(manager.Config{
		Addr:      *addr,
		DataPath:  *data,
		AgentDir:  *agentDir,
		PublicURL: *publicURL,
	})
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}
