package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/ylyt_bot/xianyu-mcp/configs"
)

func main() {
	var (
		headless bool
		binPath  string
		port     string
	)

	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&port, "port", ":18061", "服务端口")
	flag.Parse()

	if binPath == "" {
		binPath = os.Getenv("ROD_BROWSER_BIN")
	}

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)

	xianyuService := NewXianyuService()
	appServer := NewAppServer(xianyuService)

	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
