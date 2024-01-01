package main

import (
	"encoding/json"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const WS_PATH = "/ws/ping/"

// TODO: update config with version stuff
const FAKE_VERSION = 5

// TODO: ping interval as config parameter
const PING_INTERVAL = 5 * time.Second

type ProbeWebsocketMessage struct {
	Message string `json:"message"`
	Version uint   `json:"version"`
}

type WebsocketClient struct {
	Conn     *websocket.Conn
	doneChan chan int
	readChan chan []byte
}

func NewWebsocketClient(server string) (*WebsocketClient, error) {
	url := url.URL{Scheme: "ws", Host: server, Path: WS_PATH}
	conn, dialMsg, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		return &WebsocketClient{}, err
	}

	log.Infof("websocket dial successful to %s: %+v", server, dialMsg)

	return &WebsocketClient{
		Conn:     conn,
		doneChan: make(chan int),
		readChan: make(chan []byte),
	}, nil
}

func (wsc *WebsocketClient) ReceiveLoop() {
	// TODO: exponential backoff?
	for {
		_, message, err := wsc.Conn.ReadMessage()
		if err != nil {
			log.Warnf("read error: %s", err)
			continue
		}
		log.Debugf("websocket recv: %s", message)
		wsc.readChan <- message
	}
}

func (wsc *WebsocketClient) Close() {
	wsc.Conn.Close()
}

// TODO: could the PING frequency be controlled by configuration?
func (wsc *WebsocketClient) Run() {
	// we want to try and reconnect if we hit an error....
	//defer func() { startWebsocketClient(server) }()
	defer wsc.Run()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go wsc.ReceiveLoop()

	ticker := time.NewTicker(PING_INTERVAL)
	defer ticker.Stop()

	// TODO: how do we detect if server went offline abruptly?
	for {
		select {
		case <-wsc.doneChan:
			// we hit the done chan if server abruptly severs connection.
			// maybe an expontential backoff would work here?
			log.Infof("DONE CHAN HIT")
			return
		case readMsg := <-wsc.readChan:
			log.Debugf("read channel msg: %s", string(readMsg))
		case <-ticker.C:
			log.Infof("sending ping")
			msg := ProbeWebsocketMessage{Message: "PING", Version: FAKE_VERSION}
			msgBytes, marshalErr := json.Marshal(&msg)
			if marshalErr != nil {
				log.Fatalf("error with marshalling websocket message: %s", marshalErr)
			}
			err := wsc.Conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				log.Warnf("websocket write err: %s", err)
				//return
			}
		case <-interrupt:
			log.Infof("websocket goroutine receieved interrupt signal")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := wsc.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Warnf("Error closing websocket channel: %s", err)
				return
			}

			os.Exit(1)
		}
	}

}

func wsReceive(ws *websocket.Conn, doneChan chan int) {
	defer close(doneChan)

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Debugf("read error: %s", err)
			return
		}
		log.Debugf("websocket recv: %s", message)
	}
}
