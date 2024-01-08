package main

import (
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

var probeTypeMap = map[string]ProbeExecutorFactory{
	"tcp": NewTCPProbeExecutor,
	"udp": NewUDPProbeExecutor,
	// TODO: ICMP
}

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

// This exists so we can fire off all probes for any given TTL and concurrently write back
// results for that batch. We might want to revist what this interface looks like to get rid
// of this...
type ProbeBatch struct {
	sync.Mutex
	hops []ProbeResponse
}

func (b *ProbeBatch) Add(response ProbeResponse) {
	b.Lock()
	b.hops = append(b.hops, response)
	b.Unlock()
}

func (b *ProbeBatch) IsFinal(target string) bool {
	for _, hop := range b.hops {
		if hop.IP.String == target {
			return true
		}
	}
	return false
}

type ProbeExecutor interface {
	Execute(target string, port uint16, count int) ([]ProbeResponse, error)
}

type ProbeExecutorFactory func(target ProbeTarget) ProbeExecutor

func updateDNSName(hop *ProbeResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	if !hop.IP.IsZero() {
		// This should return multiple DNS names but we are only
		// expecting 1 in the data model on the server side.
		// TODO: support multiple reverse lookup records?
		names, lookupErr := net.LookupAddr(hop.IP.ValueOrZero())
		if lookupErr != nil {
			log.Debug("Reverse lookup failed for ", hop.IP)
		}

		log.Debug("Reverse lookup results: ", names)
		if len(names) > 0 {
			log.Debug(null.StringFrom(names[0]))
			hop.DNSName = null.StringFrom(names[0])
		}
	}
}

func probeHandler(target ProbeTarget) {
	log.Infof("launching probe towards %s", target.Destination)
	probe := Probe{
		Target:    target.Destination,
		StartTime: time.Now(),
		Hops:      make([]ProbeResponse, 0),
	}

	// TODO: better factory-ish thing here
	executorFactory, ok := probeTypeMap[target.Type]
	if !ok {
		log.WithFields(log.Fields{"target": target}).Warn("Unsupported target protocol")
		return
	}
	executor := executorFactory(target)
	hops, hopsErr := executor.Execute(target.Destination, target.Port, target.ProbeCount)
	if hopsErr != nil {
		log.Warn("Error executing UDP probe: ", hopsErr)
		return
	}
	probe.Hops = hops

	var wg sync.WaitGroup
	wg.Add(len(probe.Hops))

	// range will make a copy of each element and pass by value, but we want the pointer
	// so we will do this the old school way.
	for i := 0; i < len(probe.Hops); i++ {
		go updateDNSName(&probe.Hops[i], &wg)
	}
	wg.Wait()

	probe.EndTime = time.Now()
	go emitProbeResults(probe)
}
