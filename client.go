package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

const (
	// 写消息允许的等待时间
	writeWait = 10 * time.Second

	// 心跳允许的等待时间
	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	// 发送消息最大的字节数
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// 客户端
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// uuid
	uuid string

	// nickname
	nickName string
}

// UserInfo 用户信息
type UserInfo struct {
	UUID     string
	NickName string
	Client   *Client
}

// 待发送消息
type Broad struct {
	Content []byte
	Rtype   int64
	Client  *Client
}

// Msg 消息体
type Msg struct {
	Code    int
	Rtype   int
	From    string
	To      string
	Content string
	User    map[string]string
	NowUID  string
}

// 读取消息体
type ReadMsg struct {
	Content  string
	FromNick string
	MsgFrom  string
	MsgTo    string
}

// 读取消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		//写入消息
		var msg Msg
		var readMsg ReadMsg
		msg.Code = 200
		msg.Rtype = 1
		msg.From = c.uuid
		msg.Content = string(message)
		json.Unmarshal([]byte(msg.Content), &readMsg)
		msg.To = readMsg.MsgTo
		msgJSON, err := json.Marshal(msg)
		var broad Broad
		broad.Content = msgJSON
		if readMsg.MsgTo == "ALL" {
			broad.Rtype = 1
		} else {
			broad.Rtype = 2
			if _, ok := c.hub.userINFO[msg.To]; ok {
				broad.Client = c.hub.userINFO[msg.To].Client
			} else {
				//
				continue
			}
		}
		if err == nil {
			c.hub.broadcast <- broad
		}
	}
}

// 写消息
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// websocket服务架设
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	//获取昵称
	nickName := r.FormValue("nick")
	client.nickName = nickName

	//分配UUID
	uid := uuid.NewV4().String()
	client.uuid = uid

	//构建用户实体
	userInfo := &UserInfo{UUID: uid, NickName: nickName, Client: client}
	hub.userINFO[uid] = userInfo

	//实验阶段每次有人进入则把所有用户UUID发送到客户端
	var msg Msg
	var users = make(map[string]string)
	for _, vlue := range hub.userINFO {
		users[vlue.UUID] = vlue.NickName
	}
	msg.User = users
	msg.Code = 200
	msg.Rtype = 2
	msg.NowUID = uid
	msgJSON, err := json.Marshal(msg)
	var broad Broad
	broad.Content = msgJSON
	broad.Rtype = 1
	if err == nil {
		client.hub.broadcast <- broad
	}
	go client.writePump()
	go client.readPump()
}
