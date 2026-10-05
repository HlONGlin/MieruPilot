package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"merit/internal/manager"
	"merit/internal/store"
)

func main() {
	addr := flag.String("addr", ":3000", "面板与 Agent 通信监听地址")
	data := flag.String("data", "data/merit.json", "数据文件路径")
	agentDir := flag.String("agent-dir", "dist", "存放 agent 二进制的目录（merit-agent-linux-amd64 等）")
	publicURL := flag.String("public-url", "", "对外访问地址，例如 http://1.2.3.4:3000，留空则自动使用请求 Host")
	panelPath := flag.String("panel-path", "", "管理面板随机访问路径，留空自动生成")
	resetAdmin := flag.Bool("reset-admin", false, "交互式重置管理员账号，保留节点数据")
	initAdmin := flag.Bool("init-admin", false, "从标准输入读取用户名和密码并初始化管理员账号")
	hasAdmin := flag.Bool("has-admin", false, "检查是否已经存在管理员账号")
	flag.Parse()

	if *resetAdmin {
		if err := resetAdministrator(*data); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *initAdmin {
		if err := initializeAdministrator(*data); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *hasAdmin {
		st, err := store.Open(*data)
		if err != nil {
			log.Fatal(err)
		}
		if st.Admin() == nil {
			os.Exit(1)
		}
		return
	}

	srv, err := manager.New(manager.Config{
		Addr:      *addr,
		DataPath:  *data,
		AgentDir:  *agentDir,
		PublicURL: *publicURL,
		PanelPath: *panelPath,
	})
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}

func resetAdministrator(path string) error {
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("打开数据文件失败: %w", err)
	}
	in := bufio.NewReader(os.Stdin)
	fmt.Print("管理员用户名: ")
	username, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取用户名失败: %w", err)
	}
	fmt.Print("管理员密码: ")
	password, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取密码失败: %w", err)
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || len(password) < 4 {
		return fmt.Errorf("用户名不能为空，密码至少 4 位")
	}
	if err := st.SetAdmin(username, password); err != nil {
		return fmt.Errorf("保存管理员账号失败: %w", err)
	}
	fmt.Println("管理员账号已更新，节点数据和随机面板地址保持不变。")
	return nil
}

func initializeAdministrator(path string) error {
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("打开数据文件失败: %w", err)
	}
	if st.Admin() != nil {
		return fmt.Errorf("管理员账号已经存在，未做修改")
	}
	in := bufio.NewReader(os.Stdin)
	username, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取用户名失败: %w", err)
	}
	password, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取密码失败: %w", err)
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || len(password) < 4 {
		return fmt.Errorf("用户名不能为空，密码至少 4 位")
	}
	if err := st.SetAdmin(username, password); err != nil {
		return fmt.Errorf("保存管理员账号失败: %w", err)
	}
	fmt.Println("管理员账号初始化成功。")
	return nil
}
