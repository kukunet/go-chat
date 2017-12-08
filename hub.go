package main

// hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Uid from clients.
	uuids map[string]string
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		uuids:      make(map[string]string),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			//当有人连接，把新链接的UUID发送给前台 暂时把它当消息写到通道就行，然后让前台去解析

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.uuids, client.uUID)
				close(client.send)
			}
			//当有人退出，把退出的相关UUID发送给前台 暂时把它当消息写到通道就行，然后让前台去解析

		case message := <-h.broadcast:
			//log.Println("当前连接：", string([]byte(message)))
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
