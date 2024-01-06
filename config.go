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
		refreshInterval: REFRESH_INTERVAL,
	}
}

func (c *VoyagerConfig) updateTargets() {
	log.Info("Updating targets from voyager server")
	/*
		targetDefinitions, targetErr := getProbeTargets()
		if targetErr != nil {
			log.Warn("Unable to update targets: ", targetErr)
			return
		}
	*/

	proberInfo, proberErr := getProbeInfo()
	if proberErr != nil {
		log.Warn("Unable to update targets: ", proberErr)
		return
	}
	c.Version = proberInfo.Version

	// destination is guarenteed to be unique
	c.lock.Lock()
	newTargetHash := make(map[string]ProbeTarget)
	for _, target := range proberInfo.Targets {
		newTargetHash[target.Destination] = target
	}

	c.targets = newTargetHash
	c.lock.Unlock()

	log.Infof("Updated local configuration to version %d", c.Version)
	log.Infof(fmt.Sprintf("New targets: %+v", newTargetHash))
}
