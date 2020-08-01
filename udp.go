package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"gopkg.in/guregu/null.v4"
	"net"
	"time"
)

type UDPProbeExecutor struct {
	ProbeTarget
}

func (u *UDPProbeExecutor) Execute(target string, port, count int) ([]ProbeResponse, error) {
	log.Info("Starting UDP probes to ", target)

	currentTTL := 1
	startingPort := 33434
	reachedDest := false
	hops := make([]ProbeResponse, 0)
	for !reachedDest {
		for i := 0; i < count; i++ {
			dst := fmt.Sprintf("%s:%d", target, startingPort)
			startingPort++

			dialerConn, dialConnErr := net.Dial("udp", dst)
			if dialConnErr != nil {
				panic(dialConnErr)
			}

			packetConn := ipv4.NewConn(dialerConn)
			packetConn.SetTTL(currentTTL)

			sentTime := time.Now()
			_, writeErr := dialerConn.Write([]byte("test"))
			if writeErr != nil {
				panic(writeErr)
			}

			probeResponse := ProbeResponse{TTL: currentTTL}

			response, lookupErr := lookupResponses(target)
			if lookupErr != nil {
				log.Info(lookupErr)

				hops = append(hops, probeResponse)

				// using defer was leaking sockets but explicitly closing them is not
				dialerConn.Close()
				continue
			}

			dialerConn.Close()

			// FOR TESTING ONLY
			thisResponse := response[0]
			rtt := thisResponse.Timestamp.Sub(sentTime)
			probeResponse.IP = null.StringFrom(thisResponse.Source.String())
			probeResponse.Time = rtt.Milliseconds()
			probeResponse.HeaderSource = thisResponse.OriginalHeader.Src
			probeResponse.HeaderDest = thisResponse.OriginalHeader.Dst
			probeResponse.Responded = true

			hops = append(hops, probeResponse)
			if thisResponse.Response.Code == 3 && !reachedDest {
				log.Debug("Received type ", thisResponse.Response.Type, ". Stopping probes.")
				reachedDest = true
			}
		}
		currentTTL++
		if currentTTL == MAX_HOPS {
			log.Info("Max hops exceeded for probe to ", target)
			break
		}
		if reachedDest {
			log.Info("Probe complete: ", target)
		}
	}

	// TODO: error handling
	return hops, nil
}
