package dev

import (
	"time"

	"github.com/sjc5/kiruna/internal/common"
)

type ChangeType string

const (
	ChangeTypeNormalCSS   ChangeType = common.CSSNormalDirName
	ChangeTypeCriticalCSS ChangeType = common.CSSCriticalDirName
	ChangeTypeOther       ChangeType = "other"
	ChangeTypeRebuilding  ChangeType = "rebuilding"
)

type Base64 = string

type RefreshFilePayload struct {
	ChangeType   ChangeType `json:"changeType"`
	CriticalCSS  Base64     `json:"criticalCss"`
	NormalCSSURL string     `json:"normalCssUrl"`
	At           time.Time  `json:"at"`
}

// Client represents a single SSE connection
type Client struct {
	id     string
	notify chan<- RefreshFilePayload
}

// ClientManager manages all SSE clients
type ClientManager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan RefreshFilePayload
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan RefreshFilePayload),
	}
}

// Start the manager to handle clients and broadcasting
func (manager *ClientManager) start() {
	for {
		select {
		case client := <-manager.register:
			manager.clients[client] = true
		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				close(client.notify)
			}
		case msg := <-manager.broadcast:
			for client := range manager.clients {
				client.notify <- msg
			}
		}
	}
}
