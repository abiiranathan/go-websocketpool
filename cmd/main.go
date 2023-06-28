package main

import (
	"log"
	"strconv"
	"time"

	"github.com/abiiranathan/websocketpool"
)

func main() {
	url := "ws://localhost:8089/ws"
	poolSize := 10

	// Create a new WebSocketPool
	pool, err := websocketpool.NewWebSocketPool(url, poolSize)
	if err != nil {
		log.Fatalf("Failed to create WebSocket pool: %v", err)
	}
	defer pool.Close()

	// Start sending messages
	go sendMessages(pool)

	// Read messages from the WebSocket connection
	readMessages(pool)
}

func sendMessages(pool *websocketpool.WebSocketPool) {
	for i := 0; i < 1000; i++ {
		message := []byte("Message " + strconv.Itoa(i))

		err := pool.Send(message)
		if err != nil {
			log.Printf("Failed to send message: %v", err)
		} else {
			log.Printf("Message sent: %s", message)
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func readMessages(pool *websocketpool.WebSocketPool) {
	for {
		_, message, err := pool.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			continue
		}
		log.Printf("Received message: %s", message)
	}
}
