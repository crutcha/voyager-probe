package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	//"sync"
	"fmt"
	"time"
)

const (
	TIMEOUT          = 30
	PROBE_COUNT      = 3
	CONCURRENCY      = 10
	MAX_HOPS         = 20
	REFRESH_INTERVAL = 1
)

var proberToken string
var voyagerServer string

func init() {
	proberToken = os.Getenv("VOYAGER_PROBE_TOKEN")
	voyagerServer = os.Getenv("VOYAGER_SERVER")

	if proberToken == "" {
		log.Fatal("VOYAGER_PROBE_TOKEN env var required but not set")
	}

	if voyagerServer == "" {
		log.Fatal("VOYAGER_SERVER env var required but not set")
	}
}

func main() {
	customFormatter := new(log.TextFormatter)

	// Yea, this is real stupid. For some reason this wants a reference timestamp?
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true

	log.SetFormatter(customFormatter)
	log.SetLevel(log.DebugLevel)
	log.Info("Starting...")

	config := NewConfig()
	currentProbers := make(map[string]chan int)
	for {
		// TODO: LOCKING IN HERE
		config.updateTargets()
		// Remove stale targets first
		for currentDest, doneChan := range currentProbers {
			if _, ok := config.targets[currentDest]; !ok {
				log.Info("Stopping prober goroutine for ", currentDest)
				delete(currentProbers, currentDest)
				doneChan <- 1
			}
		}

		// Spin up new threads for new probes
		for destination, _ := range config.targets {
			if _, ok := currentProbers[destination]; !ok {
				log.Info("Starting prober goroutine for ", destination)
				done := make(chan int, 1)
				currentProbers[destination] = done
				go func(destination string, done chan int) {
					ticker := time.NewTicker(time.Duration(config.targets[destination].Interval) * time.Second)
					currentTickTime := config.targets[destination].Interval
					for {
						select {
						case <-ticker.C:
							log.Info("Ticked for: ", destination)
							if config.targets[destination].Interval != currentTickTime {
								log.Info(fmt.Sprintf(
									"Interval update received. Changing interval for %s from %d  to %d seconds\n", destination,
									currentTickTime, config.targets[destination].Interval,
								))
								currentTickTime = config.targets[destination].Interval
								ticker.Stop()
								ticker = time.NewTicker(time.Duration(config.targets[destination].Interval) * time.Second)
							}
						case <-done:
							log.Debug("Received halt request on done channel. Stopping ", destination)
							return
						}
					}
				}(destination, done)
			}
		}
		time.Sleep(REFRESH_INTERVAL * time.Minute)
	}

	//startICMPListener()
	/*
		var wg sync.WaitGroup
		//targets := []string{"8.8.8.8", "1.1.1.1", "squareup.com", "order.dominos.com"}
		//targets := []string{"1.1.1.1", "8.8.8.8"}

			for _, target := range targets {
				wg.Add(1)
				go probeHandler(target, &wg)
			}
			wg.Wait()
	*/
	time.Sleep(19 * time.Minute)
}
