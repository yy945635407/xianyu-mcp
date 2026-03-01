package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/ylyt_bot/xianyu-mcp/cookies"
)

func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}

	logrus.Errorf("%s %s %s %d", c.Request.Method, c.Request.URL.Path, c.GetString("account"), statusCode)
	c.JSON(statusCode, response)
}

func respondSuccess(c *gin.Context, data any, message string) {
	response := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	logrus.Infof("%s %s %s %d", c.Request.Method, c.Request.URL.Path, c.GetString("account"), http.StatusOK)
	c.JSON(http.StatusOK, response)
}

func (s *AppServer) checkLoginStatusHandler(c *gin.Context) {
	status, err := s.xianyuService.CheckLoginStatus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED", "检查登录状态失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, status, "检查登录状态成功")
}

func (s *AppServer) getLoginQrcodeHandler(c *gin.Context) {
	result, err := s.xianyuService.GetLoginQrcode(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LOGIN_QRCODE_FAILED", "获取登录二维码失败", err.Error())
		return
	}

	respondSuccess(c, result, "获取登录二维码成功")
}

func (s *AppServer) deleteCookiesHandler(c *gin.Context) {
	err := s.xianyuService.DeleteCookies(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_COOKIES_FAILED", "删除 cookies 失败", err.Error())
		return
	}

	cookiePath := cookies.GetCookiesFilePath()
	respondSuccess(c, map[string]any{
		"cookie_path": cookiePath,
		"message":     "Cookies 已成功删除，登录状态已重置。",
	}, "删除 cookies 成功")
}

func healthHandler(c *gin.Context) {
	respondSuccess(c, map[string]any{
		"status":    "healthy",
		"service":   "xianyu-mcp",
		"timestamp": time.Now().Format(time.RFC3339),
	}, "服务正常")
}
