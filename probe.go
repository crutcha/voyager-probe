package main

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
	"net"
	"sync"
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

type ProbeExecutor interface {
	Execute(target string, port, count int) ([]ProbeResponse, error)
}

func updateDNSName(hop *ProbeResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	if !hop.IP.IsZero() {
		// This should return multiple DNS names but we are only
		// expecting 1 in the data model on the server side.
		// TODO: support multiple reverse lookup records?
		names, lookupErr := net.LookupAddr(hop.IP.ValueOrZero())
		if lookupErr != nil {
			log.Info("Reverse lookup failed for ", hop.IP)
		}

		log.Debug("Reverse lookup results: ", names)
		if len(names) > 0 {
			log.Debug(null.StringFrom(names[0]))
			hop.DNSName = null.StringFrom(names[0])
		}
	}
}

func probeHandler(target ProbeTarget) {
	probe := Probe{
		Target:    target.Destination,
		StartTime: time.Now(),
		Hops:      make([]ProbeResponse, 0),
	}

	// TODO: better factory-ish thing here
	if target.Type == "udp" {
		executor := UDPProbeExecutor{target}
		hops, hopsErr := executor.Execute(target.Destination, target.Port, target.ProbeCount)
		if hopsErr != nil {
			log.Warn("Error executing UDP probe: ", hopsErr)
			return
		}
		probe.Hops = hops
	} else {
		log.Warn("Unsupported target protocol")
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(probe.Hops))

	// range will make a copy of each element and pass by value, but we want the pointer
	// so we will do this the old school way.
	for i := 0; i < len(probe.Hops); i++ {
		go updateDNSName(&probe.Hops[i], &wg)
	}
	wg.Wait()

	go emitProbeResults(probe)
}
