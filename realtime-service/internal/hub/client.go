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
