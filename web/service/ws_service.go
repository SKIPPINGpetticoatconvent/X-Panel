package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"x-ui/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WsService struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
	upgrader   websocket.Upgrader
}

// Global WsService instance (singleton pattern for simplicity in integration)
var wsService *WsService
var wsOnce sync.Once

func GetWsService() *WsService {
	wsOnce.Do(func() {
		wsService = NewWsService()
		go wsService.run()
	})
	return wsService
}

func NewWsService() *WsService {
	return &WsService{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *WsService) run() {
	for {
		select {
		case client := <-s.register:
			s.mutex.Lock()
			s.clients[client] = true
			s.mutex.Unlock()
			logger.Debug("New WebSocket client connected")

		case client := <-s.unregister:
			s.ifClientExists(client, func() {
				s.mutex.Lock()
				delete(s.clients, client)
				s.mutex.Unlock()
				client.Close()
				logger.Debug("WebSocket client disconnected")
			})

		case message := <-s.broadcast:
			s.mutex.RLock()
			for client := range s.clients {
				select {
				case <-time.After(100 * time.Millisecond):
					// Skip slow clients to avoid blocking
					logger.Warning("Skipping slow WebSocket client")
				default:
					err := client.WriteMessage(websocket.TextMessage, message)
					if err != nil {
						logger.Warningf("WebSocket write error: %v, closing connection", err)
						client.Close()
						go func(c *websocket.Conn) { s.unregister <- c }(client)
					}
				}
			}
			s.mutex.RUnlock()
		}
	}
}

func (s *WsService) ifClientExists(client *websocket.Conn, fn func()) {
	s.mutex.RLock()
	_, ok := s.clients[client]
	s.mutex.RUnlock()
	if ok {
		fn()
	}
}

func (s *WsService) HandleConnection(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warningf("Failed to upgrade WebSocket connection: %v", err)
		return
	}
	s.register <- conn

	// Keep connection alive/check for close
	go func() {
		defer func() {
			s.unregister <- conn
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Warningf("WebSocket unexpected close: %v", err)
				}
				break
			}
		}
	}()
}

func (s *WsService) Broadcast(data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("WebSocket broadcast marshal error: %v", err)
		return
	}
	s.broadcast <- bytes
}
