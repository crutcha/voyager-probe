package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"sync"
	"time"
)

const (
	PROBE_LOOKUP_TIMEOUT  = 2
	ICMP_STALE_AFTER      = 60
	ICMP_CLEANUP_INTERVAL = 60
)

// TODO: cleanup on some interval or will potentially grow unchecked if we receive ICMP
// traffic not meant for us on our socket?
var received = ResponseMap{responses: map[string][]ICMPResponse{}}

type ResponseMap struct {
	lock      sync.Mutex
	responses map[string][]ICMPResponse
}

type ICMPResponse struct {
	Response       *icmp.Message
	OriginalHeader *ipv4.Header
	Source         net.Addr
	Timestamp      time.Time
}

func startICMPListener() {
	log.Info("Starting ICMP listener thread")

	recvBuffer := make([]byte, 1514)

	go func() {
		icmpConn, connErr := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if connErr != nil {
			log.Warn(connErr)
		}

		// TODO: context handler to ensure cleanup of socket
		defer icmpConn.Close()

		for {
			_, thisSrc, recvErr := icmpConn.ReadFrom(recvBuffer)
			if recvErr != nil {
				log.Warn(recvErr)
			}
			timestamp := time.Now()

			icmpMessage, parseErr := icmp.ParseMessage(1, recvBuffer)
			if parseErr != nil {
				log.Warn(parseErr)
				continue
			}

			icmpBody, bodyErr := icmpMessage.Body.Marshal(1)
			if bodyErr != nil {
				log.Warn(bodyErr)
				continue
			}

			// Account for 4 "unused" bytes in ICMP message
			originalHeader, headerErr := ipv4.ParseHeader(icmpBody[4:24])
			if headerErr != nil {
				panic(headerErr)
			}

			response := ICMPResponse{
				Response:       icmpMessage,
				OriginalHeader: originalHeader,
				Source:         thisSrc,
				Timestamp:      timestamp,
			}

			// We will lock based on destination of probe attempts so keying on this should be
			// guarenteed to be unique
			resultKey := originalHeader.Dst.String()
			received.lock.Lock()
			if _, exists := received.responses[resultKey]; !exists {
				received.responses[resultKey] = []ICMPResponse{response}
			} else {
				received.responses[resultKey] = append(received.responses[resultKey], response)
			}
			received.lock.Unlock()
			debug := fmt.Sprintf("%+v", icmpMessage)
			log.WithFields(log.Fields{"src": thisSrc}).Debug(debug)
		}
	}()
}

// Channels with contexts dont really work here. Since we're still reliant on every
// reply coming back to the same ICMP socket, there's no way to guarentee that any
// message coming back through the channel is ACTUALLY for data we care about...
func lookupResponses(dst string) ([]ICMPResponse, error) {
	var lookupValues []ICMPResponse
	var err error
	timeout := time.Now().Add(PROBE_LOOKUP_TIMEOUT * time.Second)
	for tstamp := range time.Tick(500 * time.Millisecond) {
		if tstamp.After(timeout) {
			err = fmt.Errorf("Response lookup timed out: %s", dst)
			break
		}

		received.lock.Lock()
		values, ok := received.responses[dst]
		if ok {
			lookupValues = values
			delete(received.responses, dst)
			received.lock.Unlock()
			break
		}
		received.lock.Unlock()
	}
	return lookupValues, err
}

// Only exists to schedule icmp response hash cleanup
func icmpCleanupHandler() {
	for {
		time.Sleep(ICMP_CLEANUP_INTERVAL * time.Second)
		removeStaleICMPResponses(&received)
	}
}

// Broken out into it's own function to make it easier to test
func removeStaleICMPResponses(responsemap *ResponseMap) {
	responsemap.lock.Lock()

	// range will give us a copy of the value not a reference
	for key, response := range responsemap.responses {
		// modifying the slice in place is a bit tricky, so just create a new one
		// for now. we may want to revisit this.
		newArray := make([]ICMPResponse, 0, len(response))
		for _, value := range response {
			timeSince := time.Now().Sub(value.Timestamp)
			isExpired := timeSince.Seconds() >= ICMP_STALE_AFTER
			if !isExpired {
				newArray = append(newArray, value)
			}
		}
		if len(newArray) == 0 {
			delete(responsemap.responses, key)
		} else {
			responsemap.responses[key] = newArray
		}
	}
	responsemap.lock.Unlock()
}
