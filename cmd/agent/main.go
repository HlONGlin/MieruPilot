package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"merit/internal/agent"
)

func main() {
	managerURL := flag.String("manager", "", "Manager 地址，例如 http://1.2.3.4:3000")
	key := flag.String("key", "", "节点 API Key")
	interval := flag.Duration("interval", 5*time.Second, "轮询失败后的重试间隔")
	mitaVersion := flag.String("mita-version", "3.38.0", "自动安装的 mita 版本")
	mitaMirror := flag.String("mita-mirror", "", "GitHub 加速前缀，例如 https://ghproxy.net")
	flag.Parse()

	a, err := agent.New(agent.Config{
		Manager:     *managerURL,
		APIKey:      *key,
		Interval:    *interval,
		MitaVersion: *mitaVersion,
		MitaMirror:  *mitaMirror,
	})
	if err != nil {
		log.Fatalf("%v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := a.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
