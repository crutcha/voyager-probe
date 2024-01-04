package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	header := http.Header{"Authorization": {fmt.Sprintf("Token %s", proberToken)}}
	conn, dialMsg, err := websocket.DefaultDialer.Dial(url.String(), header)
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)

			}
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

func startWebsocketLoop(server string) {
	delay := 1 * time.Second
	for {
		log.Infof("websocket creation backoff timer: %d", delay)
		time.Sleep(delay)
		clientErr := startWebsocketClient(server)
		if clientErr != nil {
			delay *= 2
			log.Warn("error in websocket client loop: %w", clientErr)
		}
	}
}

// TODO: could the PING frequency be controlled by configuration?
func startWebsocketClient(server string) error {
	url := url.URL{Scheme: "ws", Host: server, Path: WS_PATH}
	header := http.Header{"Authorization": {fmt.Sprintf("Token %s", proberToken)}}
	conn, dialMsg, err := websocket.DefaultDialer.Dial(url.String(), header)
	if err != nil {
		return fmt.Errorf("error creating websocket connection: %s", err)
	}

	log.Infof("websocket dial successful to %s: %+v", server, dialMsg)
	// we want to try and reconnect if we hit an error....
	defer func() {
		log.Info("DEFERRED CONN CLOSE")
		conn.Close()
	}()

	interrupt := make(chan os.Signal, 1)
	done := make(chan int)
	readChan := make(chan string)
	signal.Notify(interrupt, os.Interrupt)
	ticker := time.NewTicker(PING_INTERVAL)
	defer func() {
		log.Info("DEFFERED TICKER STOP")
		ticker.Stop()
	}()
	go wsReceive(conn, done, readChan)

	// TODO: how do we detect if server went offline abruptly?
	for {
		select {
		case <-done:
			// we hit the done chan if server abruptly severs connection.
			// maybe an expontential backoff would work here?
			log.Infof("DONE CHAN HIT")
			return fmt.Errorf("read channel closed")
		case readMsg := <-readChan:
			log.Debugf("read channel msg: %s", string(readMsg))
		case <-ticker.C:
			log.Infof("sending ping")
			msg := ProbeWebsocketMessage{Message: "PING", Version: FAKE_VERSION}
			msgBytes, marshalErr := json.Marshal(&msg)
			if marshalErr != nil {
				log.Fatalf("error with marshalling websocket message: %s", marshalErr)
			}
			err := conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				return fmt.Errorf("websocket write err: %w", err)
			}
		case <-interrupt:
			log.Infof("websocket goroutine receieved interrupt signal")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Warnf("Error closing websocket channel: %s", err)
				return nil
			}

			// TODO: we might need to cleanly shut down other things too so eventually remove
			// the exit here
			os.Exit(1)
		}
	}

}

func wsReceive(ws *websocket.Conn, doneChan chan int, readChan chan string) {
	defer close(doneChan)

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Debugf("read error: %s", err)
			return
		}
		log.Debugf("websocket recv: %s", message)
		readChan <- string(message)
	}
}
