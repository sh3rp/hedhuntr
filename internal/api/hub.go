package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Type       string         `json:"type"`
	Topic      string         `json:"topic,omitempty"`
	EventID    string         `json:"event_id,omitempty"`
	EventType  string         `json:"event_type,omitempty"`
	OccurredAt string         `json:"occurred_at,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

type Client struct {
	conn   *websocket.Conn
	hub    *Hub
	topics map[string]struct{}
	send   chan WSMessage
}

func NewHub() *Hub {
	return &Hub{clients: map[*Client]struct{}{}}
}

func (h *Hub) Add(conn *websocket.Conn) *Client {
	client := &Client{
		conn:   conn,
		hub:    h,
		topics: map[string]struct{}{},
		send:   make(chan WSMessage, 16),
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *Hub) Remove(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(message WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if !client.subscribed(message.Topic) {
			continue
		}
		select {
		case client.send <- message:
		default:
		}
	}
}

func (c *Client) subscribed(topic string) bool {
	if topic == "" {
		return true
	}
	_, ok := c.topics[topic]
	return ok || len(c.topics) == 0
}

func (c *Client) writeLoop(pingInterval time.Duration) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

func (c *Client) readLoop() {
	defer c.hub.Remove(c)
	defer c.conn.Close()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var message struct {
			Type   string   `json:"type"`
			Topics []string `json:"topics"`
		}
		if err := json.Unmarshal(data, &message); err != nil {
			continue
		}
		if message.Type == "subscribe" {
			c.topics = map[string]struct{}{}
			for _, topic := range message.Topics {
				c.topics[topic] = struct{}{}
			}
		}
	}
}
