package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"io/ioutil"
	"net"
	"sync"
	"time"
)

type Probe struct {
	Dest      string          `json:"dest"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
	Responses []ProbeResponse `json:"responses"`
}

type ProbeResponse struct {
	ProbeSource  string `json:"probe_source"`
	Time         int64  `json:"response_time"`
	TTL          int    `json:"ttl"`
	HeaderSource net.IP `json:"header_source"`
	HeaderDest   net.IP `json:"header_dest"`
}

func probeHandler(dst string, wg *sync.WaitGroup) {
	log.Info("Starting probes to ", dst)
	probe := Probe{
		Dest:      dst,
		StartTime: time.Now(),
		Responses: make([]ProbeResponse, 0),
	}

	sequence := 0
	currentTTL := 1
	startingPort := 33434
	reachedDest := false
	for !reachedDest {
		for i := 0; i < PROBE_COUNT; i++ {
			dst := fmt.Sprintf("%s:%d", probe.Dest, startingPort)
			sequence++
			startingPort++

			dialerConn, dialConnErr := net.Dial("udp", dst)
			if dialConnErr != nil {
				panic(dialConnErr)
			}
			defer dialerConn.Close()

			packetConn := ipv4.NewConn(dialerConn)
			packetConn.SetTTL(currentTTL)

			sentTime := time.Now()
			_, writeErr := dialerConn.Write([]byte("test"))
			if writeErr != nil {
				panic(writeErr)
			}

			probeResponse := ProbeResponse{TTL: currentTTL}

			response, lookupErr := lookupResponses(probe.Dest)
			if lookupErr != nil {
				log.Info(lookupErr)

				// TODO: dirty. remove.
				probe.Responses = append(probe.Responses, probeResponse)

				continue
			}

			// FOR TESTING ONLY
			thisResponse := response[0]
			rtt := thisResponse.Timestamp.Sub(sentTime)
			probeResponse.ProbeSource = thisResponse.Source.String()
			probeResponse.Time = rtt.Milliseconds()
			probeResponse.HeaderSource = thisResponse.OriginalHeader.Src
			probeResponse.HeaderDest = thisResponse.OriginalHeader.Dst

			probe.Responses = append(probe.Responses, probeResponse)
			if thisResponse.Response.Code == 3 && !reachedDest {
				log.Debug("Received type ", thisResponse.Response.Type, ". Stopping probes.")
				reachedDest = true
			}
		}
		currentTTL++
		if currentTTL == MAX_HOPS {
			log.Info("Max hops exceeded for probe to ", probe.Dest)
			break
		}
		if reachedDest {
			log.Info("Probe complete: ", probe.Dest)
		}
	}
	probe.EndTime = time.Now()
	output, _ := json.Marshal(probe)
	_ = ioutil.WriteFile("outputs/"+dst+".json", output, 0644)
	wg.Done()
}
