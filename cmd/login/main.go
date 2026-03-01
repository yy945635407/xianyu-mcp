package main

import (
	"context"
	"encoding/json"
	"flag"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/ylyt_bot/xianyu-mcp/browser"
	"github.com/ylyt_bot/xianyu-mcp/cookies"
	"github.com/ylyt_bot/xianyu-mcp/xianyu"
)

func main() {
	var binPath string
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.Parse()

	b := browser.NewBrowser(false, browser.WithBinPath(binPath))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewLogin(page)

	status, username, err := action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status: %v", err)
	}
	logrus.Infof("当前登录状态: %v, 用户: %s", status, username)

	if status {
		return
	}

	logrus.Info("开始登录流程，请在浏览器中手动扫码/登录...")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	if err = action.Login(ctx); err != nil {
		logrus.Fatalf("登录失败: %v", err)
	}

	if err := saveCookies(page); err != nil {
		logrus.Fatalf("failed to save cookies: %v", err)
	}

	status, username, err = action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to re-check login status: %v", err)
	}

	if status {
		logrus.Infof("登录成功，当前用户: %s", username)
	} else {
		logrus.Error("登录流程完成但仍未登录")
	}
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}
