package main

import "encoding/json"

// Hub 核心结构体
type Hub struct {
	// 注册客户端
	clients map[*Client]bool

	// 消息通道
	broadcast chan Broad

	register chan *Client

	// 卸载客户端
	unregister chan *Client

	// Uid from clients.
	userINFO map[string]*UserInfo
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan Broad),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		userINFO:   make(map[string]*UserInfo),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.userINFO, client.uuid)
				close(client.send)
			}
			//更新客户端列表
			var msg Msg
			var users = make(map[string]string)
			for _, vlue := range h.userINFO {
				users[vlue.UUID] = vlue.NickName
			}
			msg.User = users
			msg.Code = 200
			msg.Rtype = 2
			msgJSON, err := json.Marshal(msg)
			if err == nil {
				for client := range h.clients {
					select {
					case client.send <- msgJSON:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
		case broad := <-h.broadcast:
			if broad.Rtype == 1 {
				for client := range h.clients {
					select {
					case client.send <- broad.Content:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
			if broad.Rtype == 2 {
				client := broad.Client
				select {
				case client.send <- broad.Content:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
