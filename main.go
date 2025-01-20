package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

type IDEMessageObjectScript struct {
	GUID   string `json:"guid"`
	Script string `json:"script"`
}

type IDEMessage struct {
	MessageID     int                     `json:"messageID"`
	ScriptStates  []*ScriptState          `json:"scriptStates,omitempty"`
	CustomMessage string                  `json:"customMessage,omitempty"`
	ObjectScript  *IDEMessageObjectScript `json:",omitempty,inline"`
}

type TTSMessage struct {
	MessageID    int            `json:"messageID"`
	ScriptStates []*ScriptState `json:"scriptStates,omitempty"`
	Message      string         `json:"message,omitempty"`
}

type SendDataBody struct {
	Operation int `json:"operation"`
}

type ScriptState struct {
	Name   string  `json:"name"`
	GUID   string  `json:"guid"`
	Script string  `json:"script"`
	UI     *string `json:"ui,omitempty"`
}

const (
	REST_PORT = "39997"
	PORT_IDE  = "39998"
	TTS_PORT  = "39999"

	TTS_MESSAGE_TYPE_NEW_OBJECT          = 0
	TTS_MESSAGE_TYPE_LOAD_GAME           = 1
	TTS_MESSAGE_TYPE_PRINT_MSG           = 2
	TTS_MESSAGE_TYPE_ERROR_MSG           = 3
	TTS_MESSAGE_TYPE_CUSTOM_MSG          = 4
	TTS_MESSAGE_TYPE_RETURN_MSG          = 5
	TTS_MESSAGE_TYPE_USER_SAVED          = 6
	TTS_MESSAGE_TYPE_USER_CREATED_OBJECT = 7

	IDE_MESSAGE_GET_ALL             = 0
	IDE_MESSAGE_SEND_SCRIPT_DATA    = 1
	IDE_MESSAGE_SEND_CUSTOM_MESSAGE = 2
	IDE_MESSAGE_EXEC_LUA_CODE       = 3
)

var (
	scriptsDir string
)

func main() {
	scriptsDir = filepath.Join(filepath.Dir(os.Args[0]), "scripts")
	err := os.MkdirAll(scriptsDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create scripts directory: %v", err)
	}

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

	go handleIDEConnections(ln)

	startRESTAPI()
}

func startRESTAPI() {
	http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var requestBody SendDataBody
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ttsConn, err := net.Dial("tcp4", "127.0.0.1:"+TTS_PORT)
		if err != nil {
			log.Printf("Could not open connection to TTS: %v", err)
			http.Error(w, "Could not open connection to TTS", http.StatusInternalServerError)
			return
		}

		ideMessage := IDEMessage{
			MessageID: requestBody.Operation,
		}
		ideMessageBytes, err := json.Marshal(ideMessage)
		if err != nil {
			log.Printf("Failed to serialize message: %v", err)
			http.Error(w, "Failed to serialize message", http.StatusInternalServerError)
			return
		}

		_, err = ttsConn.Write(ideMessageBytes)
		if err != nil {
			log.Printf("Failed to send message to TTS: %v", err)
			http.Error(w, "Failed to send message to TTS", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message sent to TTS"))
	})

	log.Printf("Starting REST API on port %s", REST_PORT)
	if err := http.ListenAndServe(":"+REST_PORT, nil); err != nil {
		log.Fatalf("Failed to start REST API: %v", err)
	}
}

func handleIDEConnections(ln net.Listener) {
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

	jsonDec := json.NewDecoder(conn)

	for {
		var rawMessage json.RawMessage
		err := jsonDec.Decode(&rawMessage)
		if err != nil {
			if err == io.EOF {
				log.Print("Connection closed by Tabletop")
			} else {
				log.Printf("Could not decode message: %v", err)
			}
			return
		}

		log.Printf("[TTS RAW MESSAGE] %v", string(rawMessage))

		var message TTSMessage
		err = json.Unmarshal(rawMessage, &message)
		if err != nil {
			log.Printf("Could not unmarshal message into TTS format: %v", err)
			continue
		}

		log.Printf("[TTS MESSAGE] %+v", message)
		handleTTSMessage(&message)
	}
}

func handleTTSMessage(message *TTSMessage) {
	if message == nil {
		log.Println("TTS message was nil")
		return
	}

	switch message.MessageID {
	case TTS_MESSAGE_TYPE_NEW_OBJECT:
		log.Print("New object message received")
		createOrUpdateScriptFiles(message.ScriptStates)
	case TTS_MESSAGE_TYPE_LOAD_GAME:
		log.Print("Load game message received")
		cleanScriptFiles()
		createOrUpdateScriptFiles(message.ScriptStates)
	}
}

func cleanScriptFiles() {
	files, err := os.ReadDir(scriptsDir)
	if err != nil {
		log.Printf("Could not read scripts directory: %v", err)
		return
	}

	for _, file := range files {
		log.Printf("Removing file '%s'", file.Name())
		err := os.Remove(filepath.Join(scriptsDir, file.Name()))
		if err != nil {
			log.Printf("Could not remove file '%s': %v", file.Name(), err)
		}
	}
}

func createOrUpdateScriptFiles(scriptFiles []*ScriptState) {
	for _, scriptFile := range scriptFiles {
		filename := objectDataToFilename(scriptFile)
		log.Printf("Creating or updating lua file %s", filename)

		luaFilepath := filepath.Join(scriptsDir, filename+".lua")
		xmlFilepath := filepath.Join(scriptsDir, filename+".xml")
		log.Printf("Attempting to create lua file at filepath: %s", luaFilepath)
		log.Printf("Attempting to create xml file at filepath: %s", xmlFilepath)

		// Lua
		file, err := os.Create(luaFilepath)
		if err != nil {
			log.Printf("Could not create or update file '%s': %v", filename, err)
			continue
		}

		_, err = file.WriteString(scriptFile.Script)
		if err != nil {
			log.Printf("Could not write script data to file '%s': %v", filename, err)
		}

		err = file.Close()
		if err != nil {
			log.Printf("Could not close file '%s': %v", filename, err)
		}

		// XML
		if scriptFile.UI == nil || *scriptFile.UI == "" {
			continue
		}

		file, err = os.Create(xmlFilepath)
		if err != nil {
			log.Printf("Could not create or update file '%s': %v", filename, err)
			continue
		}

		_, err = file.WriteString(*scriptFile.UI)
		if err != nil {
			log.Printf("Could not write UI data to file '%s': %v", filename, err)
		}

		err = file.Close()
		if err != nil {
			log.Printf("Could not close file '%s': %v", filename, err)
		}
	}
}

func objectDataToFilename(scriptState *ScriptState) string {
	return fmt.Sprintf("%s_%s", scriptState.Name, scriptState.GUID)
}
