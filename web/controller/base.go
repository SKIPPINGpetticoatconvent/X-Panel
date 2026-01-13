package controller

import (
	"net/http"
	"strings"

	"x-ui/logger"
	"x-ui/web/locale"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type BaseController struct{}

func (a *BaseController) checkLogin(c *gin.Context) {
	if !session.IsLogin(c) {
		// 【安全增强】: 隐身模式 - 对于 API 请求，未授权直接返回 404 Not Found
		// 这可以防止外部扫描器探测到面板 API 的存在
		if strings.Contains(c.Request.RequestURI, "/api/") {
			pureJsonMsg(c, http.StatusNotFound, false, "404 page not found")
			c.Abort()
			return
		}

		if isAjax(c) {
			pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
	} else {
		c.Next()
	}
}

func I18nWeb(c *gin.Context, name string, params ...string) string {
	anyfunc, funcExists := c.Get("I18n")
	if !funcExists {
		logger.Warning("I18n function not exists in gin context!")
		return ""
	}
	i18nFunc, _ := anyfunc.(func(i18nType locale.I18nType, key string, keyParams ...string) string)
	msg := i18nFunc(locale.Web, name, params...)
	return msg
}
