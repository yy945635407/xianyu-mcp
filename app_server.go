package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

type AppServer struct {
	xianyuService *XianyuService
	mcpServer     *mcp.Server
	router        *gin.Engine
	httpServer    *http.Server
}

func NewAppServer(xianyuService *XianyuService) *AppServer {
	appServer := &AppServer{
		xianyuService: xianyuService,
	}

	appServer.mcpServer = InitMCPServer(appServer)
	return appServer
}

func (s *AppServer) Start(port string) error {
	s.router = setupRoutes(s)
	s.httpServer = &http.Server{
		Addr:    port,
		Handler: s.router,
	}

	go func() {
		logrus.Infof("启动 HTTP 服务器: %s", port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("服务器启动失败: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logrus.Warnf("等待连接关闭超时，强制退出: %v", err)
	} else {
		logrus.Info("服务器已优雅关闭")
	}

	return nil
}
