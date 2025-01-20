package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

type TTSMessage struct {
	MessageID    int            `json:"messageID"`
	ScriptStates []*ScriptState `json:"scriptStates,omitempty"`
	Message      string         `json:"message,omitempty"`
}

type ScriptState struct {
	Name   string `json:"name"`
	GUID   string `json:"guid"`
	Script string `json:"script"`
}

const (
	PORT_IDE = "39998"
	TTS_PORT = "39999"

	MESSAGE_TYPE_NEW_OBJECT          = 0
	MESSAGE_TYPE_LOAD_GAME           = 1
	MESSAGE_TYPE_PRINT_MSG           = 2
	MESSAGE_TYPE_ERROR_MSG           = 3
	MESSAGE_TYPE_CUSTOM_MSG          = 4
	MESSAGE_TYPE_RETURN_MSG          = 5
	MESSAGE_TYPE_USER_SAVED          = 6
	MESSAGE_TYPE_USER_CREATED_OBJECT = 7
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
	case MESSAGE_TYPE_NEW_OBJECT:
		log.Print("New object message received")
		createOrUpdateLuaFiles(message.ScriptStates)
	case MESSAGE_TYPE_LOAD_GAME:
		log.Print("Load game message received")
		cleanScriptFiles()
		createOrUpdateLuaFiles(message.ScriptStates)
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

func createOrUpdateLuaFiles(luaScripts []*ScriptState) {
	for _, luaScript := range luaScripts {
		filename := objectDataToFilename(luaScript)
		log.Printf("Creating or updating lua file %s", filename)

		luaFilepath := filepath.Join(scriptsDir, filename+".lua")
		log.Printf("Attempting to create file at filepath: %s", luaFilepath)
		file, err := os.Create(luaFilepath)
		if err != nil {
			log.Printf("Could not create or update file '%s': %v", filename, err)
			continue
		}

		_, err = file.WriteString(luaScript.Script)
		if err != nil {
			log.Printf("Could not write script data to file '%s': %v", filename, err)
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
