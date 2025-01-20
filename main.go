package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"strings"
)

type TTSMessage struct {
	MessageID    string        `json:"messageID"`
	ScriptStates []ScriptState `json:"scriptStates,omitempty"`
	Message      string        `json:"message,omitempty"`
}

type ScriptState struct {
	GUID   string `json:"guid"`
	Script string `json:"script"`
}

const (
	PORT_IDE = "39998"
	TTS_PORT = "39999"
)

func main() {
	ln, err := net.Listen("tcp4", "127.0.0.1:"+PORT_IDE)
	if err != nil {
		log.Fatalf("Could not listen on IDE's port: %v", err)
	}
	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatalf("Could not close listener: %v", err)
		}
	}(ln)

	log.Printf("Listening on %s", ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Could not accept connection: %v", err)
			continue
		}
		go handleIDEConnection(conn)
	}
}

func handleIDEConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Could not close connection: %v", err)
		}
	}(conn)

	log.Printf("Started handling connection from %s", conn.RemoteAddr().String())

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Received: %s", line)
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		var msg TTSMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Could not unmarshal line '%s' as message: %v", line, err)
			continue
		}

		log.Printf("[TTS MESSAGE] %v", msg)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}
