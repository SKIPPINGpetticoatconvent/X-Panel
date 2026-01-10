package controller

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"time"

	"x-ui/util/crypto"
	"x-ui/web/entity"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

type SettingController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
	certService    *service.CertService
}

func NewSettingController(g *gin.RouterGroup, certService *service.CertService) *SettingController {
	a := &SettingController{
		certService: certService,
	}
	a.initRouter(g)
	return a
}

func (a *SettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/setting")

	g.POST("/all", a.getAllSetting)
	g.POST("/defaultSettings", a.getDefaultSettings)
	g.POST("/update", a.updateSetting)
	g.POST("/updateUser", a.updateUser)
	g.POST("/restartPanel", a.restartPanel)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.POST("/cert/apply", a.applyIPCert)
	g.GET("/cert/status", a.getCertStatus)
}

func (a *SettingController) getAllSetting(c *gin.Context) {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, allSetting, nil)
}

func (a *SettingController) getDefaultSettings(c *gin.Context) {
	result, err := a.settingService.GetDefaultSettings(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, result, nil)
}

func (a *SettingController) updateSetting(c *gin.Context) {
	allSetting := &entity.AllSetting{}
	err := c.ShouldBind(allSetting)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	err = a.settingService.UpdateAllSetting(allSetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

func (a *SettingController) updateUser(c *gin.Context) {
	form := &updateUserForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || !crypto.CheckPasswordHash(user.Password, form.OldPassword) {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		user.Username = form.NewUsername
		user.Password, _ = crypto.HashPasswordAsBcrypt(form.NewPassword)
		session.SetLoginUser(c, user)
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

func (a *SettingController) restartPanel(c *gin.Context) {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanelSuccess"), err)
}

func (a *SettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

type applyIPCertForm struct {
	Email string `json:"email" binding:"required"`
	IP    string `json:"targetIp" binding:"required"`
}

func (a *SettingController) applyIPCert(c *gin.Context) {
	var form applyIPCertForm
	err := c.ShouldBindJSON(&form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}

	// 保存配置
	err = a.settingService.SetIpCertEnable(true)
	if err != nil {
		jsonMsg(c, "Failed to enable IP cert", err)
		return
	}

	err = a.settingService.SetIpCertEmail(form.Email)
	if err != nil {
		jsonMsg(c, "Failed to set IP cert email", err)
		return
	}

	err = a.settingService.SetIpCertTarget(form.IP)
	if err != nil {
		jsonMsg(c, "Failed to set IP cert target", err)
		return
	}

	// 申请证书
	err = a.certService.ObtainIPCert(form.IP, form.Email)
	if err != nil {
		jsonMsg(c, "Failed to obtain IP certificate", err)
		return
	}

	jsonMsg(c, "IP certificate obtained successfully", nil)
}

type certStatus struct {
	Enabled        bool   `json:"enabled"`
	TargetIp       string `json:"targetIp"`
	CertPath       string `json:"certPath"`
	CertExists     bool   `json:"certExists"`
	NotBefore      string `json:"notBefore,omitempty"`
	NotAfter       string `json:"notAfter,omitempty"`
	Issuer         string `json:"issuer,omitempty"`
	Subject        string `json:"subject,omitempty"`
	DaysRemaining  int    `json:"daysRemaining,omitempty"`
}

func (a *SettingController) getCertStatus(c *gin.Context) {
	status := &certStatus{}

	// 获取启用状态
	enabled, err := a.settingService.GetIpCertEnable()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	status.Enabled = enabled

	// 获取目标 IP
	targetIp, err := a.settingService.GetIpCertTarget()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	status.TargetIp = targetIp

	// 获取证书路径
	certPath, err := a.settingService.GetIpCertPath()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	status.CertPath = certPath

	// 检查证书是否存在并解析信息
	if certPath != "" {
		certFile := certPath + ".crt"
		keyFile := certPath + ".key"

		if _, err := os.Stat(certFile); err == nil {
			status.CertExists = true

			// 加载并解析证书
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err == nil && len(cert.Certificate) > 0 {
				parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
				if err == nil {
					status.NotBefore = parsedCert.NotBefore.Format("2006-01-02 15:04:05")
					status.NotAfter = parsedCert.NotAfter.Format("2006-01-02 15:04:05")
					status.Issuer = parsedCert.Issuer.CommonName
					status.Subject = parsedCert.Subject.CommonName
					duration := time.Until(parsedCert.NotAfter)
					status.DaysRemaining = int(duration.Hours() / 24)
				}
			}
		}
	}

	jsonObj(c, status, nil)
}
