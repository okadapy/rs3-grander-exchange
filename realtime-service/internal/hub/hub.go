package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	UserID   uint
	Username string

	mu          sync.RWMutex
	subscribed  map[int64]struct{}
}

func (c *Client) Subscribe(id int64) {
	c.mu.Lock(); defer c.mu.Unlock()
	if c.subscribed == nil { c.subscribed = map[int64]struct{}{} }
	c.subscribed[id] = struct{}{}
}
func (c *Client) Unsubscribe(id int64) {
	c.mu.Lock(); defer c.mu.Unlock()
	delete(c.subscribed, id)
}
func (c *Client) IsSubscribed(id int64) bool {
	c.mu.RLock(); defer c.mu.RUnlock()
	_, ok := c.subscribed[id]
	return ok
}

type Hub struct {
	log        *zap.Logger
	clients    map[*Client]struct{}
	byItem     map[int64]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan Envelope // for global chat
	mu         sync.RWMutex
}

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func New(log *zap.Logger) *Hub {
	return &Hub{
		log:        log,
		clients:    map[*Client]struct{}{},
		byItem:     map[int64]map[*Client]struct{}{},
		register:   make(chan *Client, 32),
		unregister: make(chan *Client, 32),
		broadcast:  make(chan Envelope, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
			h.log.Debug("client registered", zap.String("user", c.Username))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.Send)
				for id, set := range h.byItem {
					delete(set, c)
					if len(set) == 0 { delete(h.byItem, id) }
				}
			}
			h.mu.Unlock()

		case env := <-h.broadcast:
			h.mu.RLock()
			clients := make([]*Client, 0, len(h.clients))
			for c := range h.clients { clients = append(clients, c) }
			h.mu.RUnlock()
			body, _ := json.Marshal(env)
			for _, c := range clients {
				select {
				case c.Send <- body:
				default:
				}
			}
		}
	}
}

// BroadcastPrice routes a price update to all subscribed clients of an item.
func (h *Hub) BroadcastPrice(itemID int64, payload []byte) {
	h.mu.RLock()
	targets := h.byItem[itemID]
	clients := make([]*Client, 0, len(targets))
	for c := range targets { clients = append(clients, c) }
	h.mu.RUnlock()

	env := Envelope{Type: "price", Payload: payload}
	body, _ := json.Marshal(env)
	for _, c := range clients {
		select {
		case c.Send <- body:
		default:
		}
	}
}

func (h *Hub) BroadcastChat(payload []byte) {
	env := Envelope{Type: "chat", Payload: payload}
	h.broadcast <- env
}

// --- client pump ---

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

// ApplySubscription toggles per-item subscription.
func (h *Hub) ApplySubscription(c *Client, ids []int64, subscribe bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range ids {
		if subscribe {
			c.Subscribe(id)
			if h.byItem[id] == nil { h.byItem[id] = map[*Client]struct{}{} }
			h.byItem[id][c] = struct{}{}
		} else {
			c.Unsubscribe(id)
			if set, ok := h.byItem[id]; ok {
				delete(set, c)
				if len(set) == 0 { delete(h.byItem, id) }
			}
		}
	}
}

func (c *Client) ReadPump(h *Hub, onMessage func(*Client, []byte)) {
	defer func() {
		h.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { return c.Conn.SetReadDeadline(time.Now().Add(pongWait)) })

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		onMessage(c, msg)
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
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
