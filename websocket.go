package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

func startWebsocketLoop(server string, config *VoyagerConfig) {
	delay := 1 * time.Second
	success := false
	for {
		log.Infof("websocket creation backoff timer: %s", delay)
		time.Sleep(delay)
		clientErr := startWebsocketClient(server, config, &success)
		if clientErr != nil {
			log.Warn("error in websocket client loop: %w", clientErr)
			if !success {
				delay *= 2
			} else {
				success = false
				delay = 1 * time.Second
			}
		}
	}
}

// TODO: could the PING frequency be controlled by configuration?
func startWebsocketClient(server string, config *VoyagerConfig, success *bool) error {
	url := url.URL{Scheme: "ws", Host: server, Path: WS_PATH}
	header := http.Header{"Authorization": {fmt.Sprintf("Token %s", proberToken)}}
	conn, dialMsg, err := websocket.DefaultDialer.Dial(url.String(), header)
	if err != nil {
		return fmt.Errorf("error creating websocket connection: %s", err)
	}

	log.Infof("websocket dial successful to %s: %+v", server, dialMsg)
	*success = true
	// we want to try and reconnect if we hit an error....
	defer conn.Close()

	done := make(chan int)
	readChan := make(chan []byte)
	ticker := time.NewTicker(PING_INTERVAL)
	defer ticker.Stop()
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
			var probeMsg ProbeWebsocketMessage
			probeUnmarshalErr := json.Unmarshal(readMsg, &probeMsg)
			if probeUnmarshalErr != nil {
				return fmt.Errorf("Error receiving websocket signal from server: %w", probeUnmarshalErr)
			}
			if probeMsg.Message == "UPDATE" {
				log.Infof("Received update signal from server for new version %d", probeMsg.Version)
				config.updateTargets()
			}
			log.Debugf("read channel msg: %s", string(readMsg))
		case <-ticker.C:
			msg := ProbeWebsocketMessage{Message: "PING", Version: config.Version}
			msgBytes, marshalErr := json.Marshal(&msg)
			if marshalErr != nil {
				log.Fatalf("error with marshalling websocket message: %s", marshalErr)
			}
			err := conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				return fmt.Errorf("websocket write err: %w", err)
			}
		}
	}

}

func wsReceive(ws *websocket.Conn, doneChan chan int, readChan chan []byte) {
	defer close(doneChan)

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Debugf("read error: %s", err)
			return
		}
		log.Debugf("websocket recv: %s", message)
		readChan <- message
	}
}
