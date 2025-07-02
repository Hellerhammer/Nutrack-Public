package messaging

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type MessageBroadcaster interface {
	Broadcast(message string)
}

type SSEBroadcaster struct{}

func (b *SSEBroadcaster) Broadcast(message string) {
	BroadcastSSEMessage(message)
}

type StandardIOBroadcaster struct{}

func (b *StandardIOBroadcaster) Broadcast(message string) {
	BroadcastStandardIOMessage(message)
}

var broadcaster MessageBroadcaster

func InitBroadcaster(useElectronIPC bool) {
	println("Initializing broadcaster with useElectronIPC:", useElectronIPC)
	if useElectronIPC {
		broadcaster = &StandardIOBroadcaster{}
	} else {
		broadcaster = &SSEBroadcaster{}
	}
}

func BroadcastMessage(message string) {
	if broadcaster != nil {
		println("Broadcasting message:", message)
		broadcaster.Broadcast(message)
	} else {
		println("No broadcaster initialized")
	}
}

// SSE

var sseClients = make(map[chan string]bool)
var sseClientsMutex sync.Mutex

func AddSSEClient(client chan string) {
	sseClientsMutex.Lock()
	sseClients[client] = true
	sseClientsMutex.Unlock()
}

func RemoveSSEClient(client chan string) {
	sseClientsMutex.Lock()
	delete(sseClients, client)
	close(client)
	sseClientsMutex.Unlock()
}

func BroadcastSSEMessage(message string) {
	sseClientsMutex.Lock()
	clientCount := len(sseClients)
	messagesSent := 0
	
	// Log client count for debugging
	if clientCount > 0 {
		println("Broadcasting to", clientCount, "clients, message:", message)
	}
	
	for client := range sseClients {
		select {
		case client <- message:
			messagesSent++
		default:
			// If we can't send immediately, the client might be slow or blocked
			println("Client channel full or blocked, removing client")
			RemoveSSEClient(client)
		}
	}
	sseClientsMutex.Unlock()
	
	if messagesSent > 0 {
		println("Message sent:", message, "to", messagesSent, "clients")
	} else if clientCount > 0 {
		println("Warning: Message not delivered to any clients:", message)
	}
}

func BroadcastStandardIOMessage(message string) {
	response := struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}{
		Type: "sse-message",
		Data: message,
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		log.Printf("Error sending SSE message: %v\n", err)
	}
}
