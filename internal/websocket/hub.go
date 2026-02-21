package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeTaskAssigned      MessageType = "task_assigned"
	MessageTypeTaskUpdated       MessageType = "task_updated"
	MessageTypeTaskStatusChanged MessageType = "task_status_changed"
	MessageTypeDeadlineRequested MessageType = "deadline_requested"
	MessageTypeDeadlineApproved  MessageType = "deadline_approved"
	MessageTypeDeadlineRejected  MessageType = "deadline_rejected"
	MessageTypeUserApproved      MessageType = "user_approved"
)

type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type Hub struct {
	// Registered clients mapped by user ID
	clients map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to specific users
	broadcast chan *BroadcastMessage

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

type BroadcastMessage struct {
	UserIDs []uuid.UUID
	Message *Message
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; !ok {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			log.Printf("Client registered: user_id=%s", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: user_id=%s", client.userID)

		case broadcast := <-h.broadcast:
			h.mu.RLock()
			messageBytes, err := json.Marshal(broadcast.Message)
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				h.mu.RUnlock()
				continue
			}

			for _, userID := range broadcast.UserIDs {
				if clients, ok := h.clients[userID]; ok {
					for client := range clients {
						select {
						case client.send <- messageBytes:
						default:
							close(client.send)
							delete(clients, client)
							if len(clients) == 0 {
								delete(h.clients, userID)
							}
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToUser(userID uuid.UUID, message *Message) {
	h.broadcast <- &BroadcastMessage{
		UserIDs: []uuid.UUID{userID},
		Message: message,
	}
}

func (h *Hub) BroadcastToUsers(userIDs []uuid.UUID, message *Message) {
	h.broadcast <- &BroadcastMessage{
		UserIDs: userIDs,
		Message: message,
	}
}

func (h *Hub) BroadcastToAll(message *Message) {
	h.mu.RLock()
	userIDs := make([]uuid.UUID, 0, len(h.clients))
	for userID := range h.clients {
		userIDs = append(userIDs, userID)
	}
	h.mu.RUnlock()

	if len(userIDs) > 0 {
		h.broadcast <- &BroadcastMessage{
			UserIDs: userIDs,
			Message: message,
		}
	}
}
