package main

import (
	"GoServerGames/internal/server"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Check password is set
	if _, err := server.GetPassword(); err != nil {
		log.Fatal("GAME_PASSWORD environment variable must be set")
	}

	sessionStore := server.NewSessionStore()
	mm := server.NewMatchmaking()

	// Start room tick processing
	go mm.StartRoomTicks()

	// Serve static files from web directory
	webDir := filepath.Join(".", "web")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		log.Fatalf("Web directory not found: %s", webDir)
	}
	log.Printf("Serving static files from: %s", webDir)
	
	// File server for static assets
	fs := http.FileServer(http.Dir(webDir))
	
	// Handle root - serve index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
		} else {
			fs.ServeHTTP(w, r)
		}
	})

	// Login endpoint
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			RoomCode string `json:"roomCode"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.RoomCode == "" {
			http.Error(w, "Room code is required", http.StatusBadRequest)
			return
		}

		authenticated, err := server.Authenticate(req.Username, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !authenticated {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		session, err := sessionStore.CreateSession(req.Username, req.RoomCode)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    session.ID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   3600, // 1 hour
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"session": session.ID,
		})
	})

	// WebSocket endpoint with session verification
	http.HandleFunc("/ws", server.HandleWebSocketWithAuth(mm, sessionStore))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("Web interface: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server error:", err)
	}
}
