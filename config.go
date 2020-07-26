package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
)

type VoyagerConfig struct {
	token           string
	server          string
	lock            sync.Mutex
	targets         map[string]ProbeTarget
	refreshInterval uint
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
		server:          voyagerServer,
		targets:         make(map[string]ProbeTarget),
		refreshInterval: REFRESH_INTERVAL,
	}
}

func (c *VoyagerConfig) updateTargets() {
	log.Info("Updating targets from voyager server")
	targetDefinitions, targetErr := getProbeTargets()
	if targetErr != nil {
		log.Warn("Unable to update targets: ", targetErr)
		return
	}

	// destination is guarenteed to be unique
	newTargetHash := make(map[string]ProbeTarget)
	for _, target := range targetDefinitions {
		newTargetHash[target.Destination] = target
	}

	c.lock.Lock()
	c.targets = newTargetHash
	c.lock.Unlock()

	log.Debug(fmt.Sprintf("New targets: %+v", newTargetHash))
}
