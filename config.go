package main

import (
	"fmt"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

type VoyagerConfig struct {
	token           string
	Server          string
	lock            sync.Mutex
	targets         map[string]ProbeTarget
	refreshInterval uint
	Version         uint

	// the ProbeTarget struct is meant to match the data model from server.
	// we will track the done signaling channels per target separately.
	doneChans map[string]chan int
}

func NewConfig() *VoyagerConfig {
	proberToken := os.Getenv("VOYAGER_PROBE_TOKEN")
	voyagerServer := os.Getenv("VOYAGER_SERVER")

	if proberToken == "" {
		log.Fatal("VOYAGER_PROBE_TOKEN env var required but not set")

	}

	if voyagerServer == "" {
		log.Fatal("VOYAGER_SERVER env var required but not set")

	}

	return &VoyagerConfig{
		token:           proberToken,
		Server:          voyagerServer,
		targets:         make(map[string]ProbeTarget),
		doneChans:       make(map[string]chan int),
		refreshInterval: REFRESH_INTERVAL,
	}
}

func (c *VoyagerConfig) updateTargets() {
	// close existing probe goroutines first
	for currentDest, doneChan := range c.doneChans {
		log.Info("Stopping prober goroutine for ", currentDest)
		delete(c.targets, currentDest)
		delete(c.doneChans, currentDest)
		doneChan <- 1
	}
	log.Info("Updating targets from voyager server")
	proberInfo, proberErr := getProbeInfo()
	if proberErr != nil {
		log.Warn("Unable to update targets: ", proberErr)
		return
	}
	c.Version = proberInfo.Version

	// destination is guarenteed to be unique
	c.lock.Lock()
	for _, target := range proberInfo.Targets {
		c.targets[target.Destination] = target
		done := make(chan int)
		c.doneChans[target.Destination] = done
	}
	c.lock.Unlock()

	log.Infof("Updated local configuration to version %d", c.Version)
	log.Infof(fmt.Sprintf("New targets: %+v", c.targets))
}
