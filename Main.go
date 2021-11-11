package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	qrterminal "github.com/mdp/qrterminal/v3"
	gonfig "github.com/tkanos/gonfig"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type Configuration struct {
	ApiUrl string
	ApiKey string
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
	config := Configuration{}
	err := gonfig.GetConf("config.json", &config)
	if err != nil {
		panic(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(config.ApiUrl, nil)
	if err != nil {
		fmt.Println("websocket err:", err)
	}

	defer conn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			//TODO: reconnect if websocket is closed
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	data := auth{"code", config.ApiKey}

	for {
		conn.WriteJSON(data)
		time.Sleep(time.Second)
	} //TODO: running forever, the following code is so not executed

	//edited whatsmeow-example from https://godocs.io/go.mau.fi/whatsmeow#example-package

	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.IsQR() {
				qrterminal.GenerateHalfBlock(string(evt), qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
