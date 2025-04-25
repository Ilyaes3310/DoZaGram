package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Content  string `json:"content"`
}

var messages []Message
var mu sync.Mutex

func sendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Request body is empty", http.StatusBadRequest)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Println("Error decoding message:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	mu.Lock()
	messages = append(messages, msg)
	mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var clientsMu sync.Mutex

func handleMessages(conn *websocket.Conn) {
	defer conn.Close()
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			clientsMu.Lock()
			delete(clients, conn)
			clientsMu.Unlock()
			break
		}

		msg := string(data)
		fmt.Println("Received message:", msg)
		broadcastMessage(msg)
	}
}

func broadcastMessage(msg string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Println("Error sending message:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error during connection upgrade:", err)
		return
	}

	go handleMessages(conn)
}

func main() {
	http.HandleFunc("/send", sendMessage)
	http.HandleFunc("/ws", handleConnections)

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}
