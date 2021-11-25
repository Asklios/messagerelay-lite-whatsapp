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
	"time"

	"github.com/Asklios/messagerelay-lite-whatsapp/util"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	qrterminal "github.com/mdp/qrterminal/v3"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type configuration struct {
	ApiUrl  string   `json:"api_url"`
	ApiKey  string   `json:"api_key"`
	WIDJIDs []string `json:"wid_jids"`
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
	connected := true

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

	groups, err := client.GetJoinedGroups()
	if err != nil {
		log.Printf("Error getting joined groups!")
	}
	for _, group := range groups {
		log.Printf("Name: %s, JID: %s", group.GroupName.Name, group.JID)
	}

	go func() {
		for connected {
			//TODO: reconnect if websocket is closed
			_, message, err := conn.ReadMessage()
			if err != nil {
				if connected {
					log.Printf("Error during websocket read: %s", err)
					conn, _, err = websocket.DefaultDialer.Dial(config.ApiUrl, nil)
					if err != nil {
						log.Fatalf("Error reconnecting Websocket: %s", err)
					}
				}

				wg.Done()
				return
			}
			var jsonMessage map[string]interface{}
			err = json.Unmarshal(message, &jsonMessage)
			if err != nil {
				log.Printf("Could not decode message: %s, error is: %s", message, err)
			}
			switch jsonMessage["type"].(string) {
			case "verified":
				log.Print("Logged into API")
			case "create":
				for _, wid := range config.WIDJIDs {
					jid, err := types.ParseJID(wid)
					if err != nil {
						fmt.Printf("Error parsing JID: %s", err)
					}
					rawMessage := jsonMessage["content"].(string)
					formatedMessage := util.ConvertHTMLToWAStyle(rawMessage)
					message := &waProto.Message{Conversation: proto.String(formatedMessage)}
					_, err = client.SendMessage(jid, jsonMessage["id"].(string), message)
					if err != nil {
						log.Printf("Error sending message: %s", err)
					}
				}
			case "delete":
				for _, wid := range config.WIDJIDs {
					jid, err := types.ParseJID(wid)
					if err != nil {
						fmt.Printf("Error parsing JID: %s", err)
					}
					_, err = client.RevokeMessage(jid, jsonMessage["id"].(string))
					if err != nil {
						log.Printf("Could not revoke message %s: %s", jsonMessage["id"].(string), err)
					}
				}
			default:
				log.Printf("Got unknown message type: %s", message)
			}
		}
	}()

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
	connected = false
	// TODO: Make this race condition proof
	<-time.After(time.Millisecond * 10)
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by user request"))
	if err != nil {
		log.Printf("Error during websocket close: %s", err)
		return
	}
	<-time.After(time.Millisecond * 10)
	conn.Close()
}
