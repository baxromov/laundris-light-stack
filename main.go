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
	keepAliveInterval   = 10 * time.Second
	connectionReadLimit = 60 * time.Second
)

func main() {
	for {
		log.Println("Attempting to connect to WebSocket server...")

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Failed to connect to WebSocket: %v. Retrying in 2 seconds...", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Println("Connected to WebSocket server")

		done := make(chan struct{})
		go keepAlive(conn, done)

		err = handleMessages(conn, done)
		if err != nil {
			log.Printf("Connection lost: %v", err)
		}

		log.Println("Disconnected. Reconnecting...")
		time.Sleep(2 * time.Second)
	}
}

func handleMessages(conn *websocket.Conn, done chan struct{}) error {
	defer close(done)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(connectionReadLimit))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(connectionReadLimit))
		return nil
	})

	for {
		var cmd Command

		err := conn.ReadJSON(&cmd)
		if err != nil {
			return fmt.Errorf("error reading message: %w", err)
		}

		log.Printf("Received command: %+v", cmd)

		err = sendHTTPRequest(cmd)
		if err != nil {
			log.Printf("Failed to process command: %v", err)
		}
	}
}

func keepAlive(conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}
			log.Println("Ping sent to server")
		}
	}
}

func sendHTTPRequest(cmd Command) error {
	apiURL := fmt.Sprintf("http://localhost:8080/api/device/gpo/light/%s?mode=%s&turnOn=%t", cmd.DeviceID, cmd.Mode, cmd.TurnOn)

	log.Printf("Sending HTTP POST to %s", apiURL)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer([]byte{}))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	log.Printf("HTTPRequest to device_id=%s was successful", cmd.DeviceID)
	return nil
}
