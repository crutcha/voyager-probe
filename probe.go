package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"gopkg.in/guregu/null.v4"
	"io/ioutil"
	"net"
	"time"
)

type Probe struct {
	Target    string          `json:"target"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
	Hops      []ProbeResponse `json:"hops"`
}

type ProbeResponse struct {
	IP           null.String `json:"ip"`
	DNSName      null.String `json:"dns_name"`
	Time         int64       `json:"response_time"`
	Responded    bool        `json:"responded"`
	TTL          int         `json:"ttl"`
	HeaderSource net.IP      `json:"-"`
	HeaderDest   net.IP      `json:"-"`
}

func probeHandler(target ProbeTarget) {
	log.Info("Starting probes to ", target.Destination)
	probe := Probe{
		Target:    target.Destination,
		StartTime: time.Now(),
		Hops:      make([]ProbeResponse, 0),
	}

	sequence := 0
	currentTTL := 1
	startingPort := 33434
	reachedDest := false
	for !reachedDest {
		for i := 0; i < PROBE_COUNT; i++ {
			dst := fmt.Sprintf("%s:%d", probe.Target, startingPort)
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

			response, lookupErr := lookupResponses(probe.Target)
			if lookupErr != nil {
				log.Info(lookupErr)

				probe.Hops = append(probe.Hops, probeResponse)

				continue
			}

			// FOR TESTING ONLY
			thisResponse := response[0]
			rtt := thisResponse.Timestamp.Sub(sentTime)
			probeResponse.IP = null.StringFrom(thisResponse.Source.String())
			probeResponse.Time = rtt.Milliseconds()
			probeResponse.HeaderSource = thisResponse.OriginalHeader.Src
			probeResponse.HeaderDest = thisResponse.OriginalHeader.Dst
			probeResponse.Responded = true

			probe.Hops = append(probe.Hops, probeResponse)
			if thisResponse.Response.Code == 3 && !reachedDest {
				log.Debug("Received type ", thisResponse.Response.Type, ". Stopping probes.")
				reachedDest = true
			}
		}
		currentTTL++
		if currentTTL == MAX_HOPS {
			log.Info("Max hops exceeded for probe to ", probe.Target)
			break
		}
		if reachedDest {
			log.Info("Probe complete: ", probe.Target)
		}
	}
	probe.EndTime = time.Now()
	output, _ := json.Marshal(probe)
	_ = ioutil.WriteFile("outputs/"+target.Destination+".json", output, 0644)
	go emitProbeResults(probe)
}
