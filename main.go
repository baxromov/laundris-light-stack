package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Command struct {
	DeviceID string `json:"device_id"`
	Mode     string `json:"mode"`
	TurnOn   bool   `json:"turnOn"`
}

const (
	wsURL               = "wss://laundirs-supply-chain-websocket.azurewebsites.net/light-stack"
	keepAliveInterval   = 3 * time.Second  // Interval to send ping messages
	connectionReadLimit = 60 * time.Second // Maximum time to wait for server response
)

func main() {
	for {
		log.Println("Attempting to connect to WebSocket server...")

		// Establish WebSocket connection
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Failed to connect to WebSocket: %v. Retrying in 2 seconds...", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Println("Connected to WebSocket server")

		// Start a goroutine to send keep-alive ping messages
		done := make(chan struct{})
		go keepAlive(conn, done)

		// Handle WebSocket messages
		err = handleMessages(conn, done)
		if err != nil {
			log.Printf("Connection lost: %v", err)
		}

		log.Println("Disconnected. Reconnecting...")
		time.Sleep(2 * time.Second) // Wait before attempting to reconnect
	}
}

// handleMessages handles incoming WebSocket messages and processes commands
func handleMessages(conn *websocket.Conn, done chan struct{}) error {
	defer close(done) // Signal the keepAlive goroutine to exit
	defer conn.Close()

	// Set a read limit duration for the connection
	conn.SetReadDeadline(time.Now().Add(connectionReadLimit))
	conn.SetPongHandler(func(appData string) error {
		// Update the read deadline when a Pong is received
		conn.SetReadDeadline(time.Now().Add(connectionReadLimit))
		return nil
	})

	for {
		var cmd Command

		// Read message from WebSocket
		err := conn.ReadJSON(&cmd)
		if err != nil {
			return fmt.Errorf("error reading message: %w", err)
		}

		log.Printf("Received command: %+v", cmd)

		// Send HTTP POST request based on the command
		err = sendHTTPRequest(cmd)
		if err != nil {
			log.Printf("Failed to process command: %v", err)
		}
	}
}

// keepAlive sends periodic ping messages to keep the WebSocket connection alive
func keepAlive(conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			// Exit the keepAlive routine when the connection is closed
			return
		case <-ticker.C:
			// Send a ping message
			err := conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}
			log.Println("Ping sent to server")
		}
	}
}

// sendHTTPRequest sends an HTTP POST request to the given API endpoint
func sendHTTPRequest(cmd Command) error {
	// Construct the request URL
	apiURL := fmt.Sprintf("http://localhost:8080/api/device/gpo/light/%s?mode=%s&turnOn=%t", cmd.DeviceID, cmd.Mode, cmd.TurnOn)

	log.Printf("Sending HTTP POST to %s", apiURL)

	// Create an HTTP POST request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer([]byte{}))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	log.Printf("HTTPRequest to device_id=%s was successful", cmd.DeviceID)
	return nil
}
