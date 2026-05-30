package biz

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
	maxMsgSize = 4096
)

type Message struct {
	Action  string          `json:"action"`
	Sender  string          `json:"sender"`
	Content json.RawMessage `json:"content"`
}

type Client struct {
	ID     string
	Name   string
	Conn   *websocket.Conn
	Room   *Room
	Send   chan []byte
	log    *log.Helper
}

type Room struct {
	ID         string
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client

	mu       sync.RWMutex
	manager  *RoomManager
	idleDone chan struct{}
	log      *log.Helper
}

func NewRoom(id string, manager *RoomManager) *Room {
	return &Room{
		ID:         id,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		manager:    manager,
		idleDone:   make(chan struct{}),
		log:        manager.log,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Room.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMsgSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Errorf("ws read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			c.log.Errorf("parse message failed: %v", err)
			continue
		}
		msg.Sender = c.Name

		enriched, _ := json.Marshal(msg)
		c.Room.Broadcast <- enriched
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (r *Room) Run() {
	idleTimer := time.NewTimer(10 * time.Minute)
	defer idleTimer.Stop()

	for {
		select {
		case client := <-r.Register:
			r.mu.Lock()
			r.Clients[client] = true
			count := len(r.Clients)
			r.mu.Unlock()

			r.broadcastSystem(Message{
				Action: "player_joined",
				Sender: client.Name,
			})
			r.log.Infof("房间 [%s] 玩家 %s 加入, 当前 %d 人", r.ID, client.Name, count)

		case client := <-r.Unregister:
			r.mu.Lock()
			if _, ok := r.Clients[client]; ok {
				delete(r.Clients, client)
				close(client.Send)
			}
			count := len(r.Clients)
			r.mu.Unlock()

			r.broadcastSystem(Message{
				Action: "player_left",
				Sender: client.Name,
			})
			r.log.Infof("房间 [%s] 玩家 %s 离开, 当前 %d 人", r.ID, client.Name, count)

			if count == 0 {
				idleTimer.Reset(10 * time.Minute)
			}

		case <-idleTimer.C:
			r.mu.RLock()
			count := len(r.Clients)
			r.mu.RUnlock()
			if count == 0 {
				r.log.Infof("房间 [%s] 空闲超时，即将销毁", r.ID)
				r.manager.RemoveRoom(r.ID)
				return
			}
			idleTimer.Reset(10 * time.Minute)

		case <-r.idleDone:
			return

		case message := <-r.Broadcast:
			r.mu.RLock()
			for client := range r.Clients {
				select {
				case client.Send <- message:
				default:
					go func(c *Client) {
						c.Room.Unregister <- c
					}(client)
				}
			}
			r.mu.RUnlock()
		}
	}
}

func (r *Room) broadcastSystem(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for client := range r.Clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
	log   *log.Helper
}

func NewRoomManager(logger log.Logger) *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
		log:   log.NewHelper(logger),
	}
}

func (rm *RoomManager) HandleWS(w http.ResponseWriter, r *http.Request, roomID string, userID string, userName string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		rm.log.Errorf("websocket 升级失败: %v", err)
		return err
	}

	rm.mu.Lock()
	room, exists := rm.rooms[roomID]
	if !exists {
		room = NewRoom(roomID, rm)
		rm.rooms[roomID] = room
		go room.Run()
	}
	rm.mu.Unlock()

	client := &Client{
		ID:   userID,
		Name: userName,
		Conn: conn,
		Room: room,
		Send: make(chan []byte, 256),
		log:  rm.log,
	}
	room.Register <- client

	go client.WritePump()
	go client.ReadPump()

	return nil
}

func (rm *RoomManager) RemoveRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if room, ok := rm.rooms[roomID]; ok {
		close(room.idleDone)
	}
	delete(rm.rooms, roomID)
	rm.log.Infof("房间 [%s] 已销毁", roomID)
}
