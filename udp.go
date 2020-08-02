package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"gopkg.in/guregu/null.v4"
	"net"
	"strings"
	"sync"
	"time"
)

func NewUDPProbeExecutor(target ProbeTarget) ProbeExecutor {
	return &UDPProbeExecutor{target}
}

type UDPProbeExecutor struct {
	ProbeTarget
}

func (u *UDPProbeExecutor) Execute(target string, port uint16, count int) ([]ProbeResponse, error) {
	log.Info("Starting UDP probes to ", target)

	// TODO: a lot of this is repetitive with TCP impl. could we abstract this away and accept a
	// callable instead?
	currentTTL := 1
	hops := make([]ProbeResponse, 0)
	for currentTTL <= MAX_HOPS {
		var probewg sync.WaitGroup
		probewg.Add(count)
		batch := ProbeBatch{hops: make([]ProbeResponse, 0, count)}
		startingPort := uint16(33434)
		for i := 0; i < count; i++ {
			// here
			go sendUDPProbe(&probewg, &batch, target, startingPort, currentTTL)
			startingPort++
		}
		probewg.Wait()

		hops = append(hops, batch.hops...)
		currentTTL++
		if batch.IsFinal(target) {
			break
		}
	}

	log.Debug("probe complete to ", target)
	return hops, nil
}

func sendUDPProbe(wg *sync.WaitGroup, batch *ProbeBatch, target string, port uint16, ttl int) {
	dst := fmt.Sprintf("%s:%d", target, port)

	dialerConn, dialConnErr := net.Dial("udp", dst)
	if dialConnErr != nil {
		log.Warn("UDP Dialer failed: ", dialConnErr)
		return
	}

	packetConn := ipv4.NewConn(dialerConn)
	packetConn.SetTTL(ttl)

	sentTime := time.Now()
	_, writeErr := dialerConn.Write([]byte("test"))
	if writeErr != nil {
		panic(writeErr)
	}

	probeResponse := ProbeResponse{TTL: ttl}

	srcPort := strings.Split(dialerConn.LocalAddr().String(), ":")[1]
	lookupKey := fmt.Sprintf("udp:%s:%s:%d", srcPort, target, port)
	response, lookupErr := lookupResponses(lookupKey)
	if lookupErr != nil {
		log.Debug(lookupErr)

		batch.Add(probeResponse)

		// using defer was leaking sockets but explicitly closing them is not
		dialerConn.Close()
		wg.Done()
		return
	}

	dialerConn.Close()

	// FOR TESTING ONLY
	rtt := response.Timestamp.Sub(sentTime)
	probeResponse.IP = null.StringFrom(response.Source.String())
	probeResponse.Time = rtt.Milliseconds()
	probeResponse.HeaderSource = response.OriginalHeader.Src
	probeResponse.HeaderDest = response.OriginalHeader.Dst
	probeResponse.Responded = true

	batch.Add(probeResponse)
	wg.Done()
}
