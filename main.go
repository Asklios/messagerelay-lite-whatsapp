package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	qrterminal "github.com/mdp/qrterminal/v3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type configuration struct {
	ApiUrl string `json:"api_url"`
	ApiKey string `json:"api_key"`
}

type auth struct {
	Type string `json:"type"`
	Code string `json:"code"`
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}

func main() {

	// load config.json
	var config configuration
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("Could not open config.json: %s", err)
	}
	decoder := json.NewDecoder(f)
	if err = decoder.Decode(&config); err != nil {
		log.Fatalf("Error decoding config.json: %s", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(config.ApiUrl, nil)
	if err != nil {
		log.Fatalf("Websocket connection error: %s", err)
	}

	defer conn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			//TODO: reconnect if websocket is closed
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Error during websocket read: %s", err)
				return
			}
			log.Printf("Received websocket message: %s", message)
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)

	data := auth{"code", config.ApiKey}

	conn.WriteJSON(data)

	//edited whatsmeow-example from https://godocs.io/go.mau.fi/whatsmeow#example-package

	dbLog := waLog.Stdout("Database", "WARN", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			log.Fatalf("Error during new client connect: %s", err)
		}
		for evt := range qrChan {
			if evt.IsQR() {
				qrterminal.GenerateHalfBlock(string(evt), qrterminal.L, os.Stdout)
			}
			if evt == whatsmeow.QRChannelSuccess {
				break
			} else {
				fmt.Printf("Login event: %s", evt)
			}
		}
		client.Connect()
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			log.Fatalf("Error during connecting already existing client: %s", err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
