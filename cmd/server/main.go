package main

import (
	"1v1/internal/server"
	"log"
	"net/http"
)

func main() {
	mm := server.NewMatchmaking()

	// Start room tick processing
	go mm.StartRoomTicks()

	http.HandleFunc("/ws", server.HandleWebSocket(mm))

	log.Println("Server starting on :8080")
	log.Println("WebSocket endpoint: ws://localhost:8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server error:", err)
	}
}

