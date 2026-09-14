#!/usr/bin/env bash
# ============================================================
#  fix-realtime-ws.sh
#  Rewrites realtime-service/internal/handler/ws.go to use
#  hub.NewClient() instead of a struct literal (which can't set
#  unexported fields). Also ensures hub/client.go exists.
#  Run from project root.
# ============================================================
set -euo pipefail

if [ ! -f go.mod ]; then
  echo "Run from project root." >&2; exit 1
fi

# ---------- 1. Ensure hub.NewClient exists ----------
cat > realtime-service/internal/hub/client.go <<'GO'
package hub

import "github.com/gorilla/websocket"

// NewClient builds a fully-initialized Client. Handlers outside this
// package must use this constructor, because `subscribed` is unexported.
func NewClient(h *Hub, conn *websocket.Conn, userID uint, username string) *Client {
	return &Client{
		Hub:        h,
		Conn:       conn,
		Send:       make(chan []byte, 64),
		UserID:     userID,
		Username:   username,
		subscribed: map[int64]struct{}{},
	}
}
GO

# ---------- 2. Rewrite ws.go cleanly ----------
cat > realtime-service/internal/handler/ws.go <<'GO'
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/realtime-service/internal/auth"
	"github.com/rs3-market/backend/realtime-service/internal/chat"
	"github.com/rs3-market/backend/realtime-service/internal/hub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(*http.Request) bool { return true },
}

type Handler struct {
	log  *zap.Logger
	h    *hub.Hub
	auth *auth.Service
	chat *chat.Service
}

func New(h *hub.Hub, a *auth.Service, c *chat.Service, log *zap.Logger) *Handler {
	return &Handler{h: h, auth: a, chat: c, log: log}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
	r.GET("/chat/history", h.history)
	r.GET("/ws", h.ws)
}

type creds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) register(c *gin.Context) {
	var in creds
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	u, err := h.auth.Register(in.Username, in.Password)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": u.ID, "username": u.Username})
}

func (h *Handler) login(c *gin.Context) {
	var in creds
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	tok, err := h.auth.Login(in.Username, in.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": tok})
}

func (h *Handler) history(c *gin.Context) {
	before := time.Time{}
	if b := c.Query("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = t
		}
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.chat.History(before, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"messages": msgs})
}

// ws upgrades the connection. JWT is supplied via ?token=...
func (h *Handler) ws(c *gin.Context) {
	token := c.Query("token")
	userID, username, err := h.auth.Verify(token)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := hub.NewClient(h.h, conn, userID, username)
	h.h.Register(client)

	go client.WritePump()
	go client.ReadPump(h.h, h.onMessage)
}

// onMessage handles subscribe/unsubscribe/chat frames.
func (h *Handler) onMessage(c *hub.Client, msg []byte) {
	var env struct {
		Action  string  `json:"action"`
		ItemIDs []int64 `json:"item_ids,omitempty"`
		Body    string  `json:"body,omitempty"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		return
	}

	switch strings.ToLower(env.Action) {
	case "subscribe":
		h.h.ApplySubscription(c, env.ItemIDs, true)
	case "unsubscribe":
		h.h.ApplySubscription(c, env.ItemIDs, false)
	case "chat":
		if !h.chat.Allow(c.UserID) {
			payload := []byte(`{"type":"error","payload":{"message":"rate limited (3s)"}}`)
			select {
			case c.Send <- payload:
			default:
			}
			return
		}
		body := strings.TrimSpace(env.Body)
		if body == "" || len(body) > 500 {
			return
		}
		m, err := h.chat.Save(c.UserID, c.Username, body)
		if err != nil {
			h.log.Warn("chat save failed", zap.Error(err))
			return
		}
		payload, _ := json.Marshal(m)
		h.h.BroadcastChat(payload)
	}
}
GO

# ---------- 3. Format & verify ----------
echo ">>> gofmt"
gofmt -w realtime-service/internal/hub/client.go \
         realtime-service/internal/handler/ws.go

echo ">>> compiling realtime-service"
if ( cd realtime-service && go build -o /tmp/_check . ); then
  rm -f /tmp/_check
  echo "  ✓ realtime-service compiles"
else
  echo "  ✗ still failing — see errors above"
  exit 1
fi

echo ""
echo "Next: make dev"
