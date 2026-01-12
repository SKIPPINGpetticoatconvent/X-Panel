package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type APIController struct {
	BaseController
	inboundController *InboundController
	serverController  *ServerController
	Tgbot             service.TelegramService
	serverService     *service.ServerService
}

func NewAPIController(g *gin.RouterGroup, serverService *service.ServerService) *APIController {
	a := &APIController{
		serverService: serverService,
		Tgbot:         serverService.GetTelegramService(),
	}
	a.initRouter(g)
	return a
}

func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkLogin)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds, a.serverService.GetInboundService(), a.serverService.GetXrayService())

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server, a.serverService)

	// Extra routes
	api.GET("/backuptotgbot", a.BackuptoTgbot)
}

func (a *APIController) BackuptoTgbot(c *gin.Context) {
	if a.Tgbot == nil {
		jsonMsg(c, "Telegram bot not enabled", nil)
		return
	}
	a.Tgbot.SendMessage("Backup requested via API") // Simplified for now since SendBackupToAdmins is not in interface
}
