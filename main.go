package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
)

const (
	TIMEOUT     = 30
	CONCURRENCY = 10
	MAX_HOPS    = 20
	// TODO: can probbably just get rid of this with the websocket implementation
	REFRESH_INTERVAL = 2
)

var proberToken string
var voyagerServer string

func main() {
	proberToken = os.Getenv("VOYAGER_PROBE_TOKEN")
	voyagerServer = os.Getenv("VOYAGER_SERVER")

	if proberToken == "" {
		log.Fatal("VOYAGER_PROBE_TOKEN env var required but not set")
	}

	if voyagerServer == "" {
		log.Fatal("VOYAGER_SERVER env var required but not set")
	}

	debugLog := flag.Bool("d", false, "debug")
	flag.Parse()
	customFormatter := new(log.TextFormatter)

	// Yea, this is real stupid. For some reason this wants a reference timestamp?
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true

	log.SetFormatter(customFormatter)

	if *debugLog == true {
		log.SetLevel(log.DebugLevel)
		// TODO: this flag isn't just for logging anymore so...update that
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	log.Info("Starting...")
	config := NewConfig()
	config.updateTargets()
	go startWebsocketLoop(voyagerServer, config)

	/*
		TODO: icmp listener is failing, gets stuck in some weird loop, need to figure out why
		WARN[2023-12-29 19:47:15] invalid connection
		WARN[2023-12-29 19:47:15] Key exists already for probe! Overwriting :0:0.0.0.0:0
		WARN[2023-12-29 19:47:15] invalid connection
		WARN[2023-12-29 19:47:15] Key exists already for probe! Overwriting :0:0.0.0.0:0
		WARN[2023-12-29 19:47:15] invalid connection
		WARN[2023-12-29 19:47:15] Key exists already for probe! Overwriting :0:0.0.0.0:0
		WARN[2023-12-29 19:47:15] invalid connection
		WARN[2023-12-29 19:47:15] Key exists already for probe! Overwriting :0:0.0.0.0:0
		WARN[2023-12-29 19:47:15] invalid connection
		WARN[2023-12-29 19:47:15] Key exists already for probe! Overwriting :0:0.0.0.0:0
	*/
	//startICMPListener()
	for {
		// Spin up new threads for new probes
		for destination, _ := range config.targets {
			log.Info("Starting prober goroutine for ", destination)
			// TODO: would there always be something at this key?
			doneChan := config.doneChans[destination]
			go func(destination string, done chan int) {
				ticker := time.NewTicker(time.Duration(config.targets[destination].Interval) * time.Second)
				currentTickTime := config.targets[destination].Interval

				// initial probe. pass by value should be fine here
				//config.lock.Lock()
				go probeHandler(config.targets[destination])
				//config.lock.Unlock()
				for {
					select {
					case <-ticker.C:
						//config.lock.Lock()
						go probeHandler(config.targets[destination])
						if config.targets[destination].Interval != currentTickTime {
							log.Info(fmt.Sprintf(
								"Interval update received. Changing interval for %s from %d  to %d seconds\n", destination,
								currentTickTime, config.targets[destination].Interval,
							))
							currentTickTime = config.targets[destination].Interval
							ticker.Stop()
							ticker = time.NewTicker(time.Duration(config.targets[destination].Interval) * time.Second)
						}
						//config.lock.Unlock()
					case <-done:
						log.Infof("Received halt request on done channel. Stopping ", destination)
						// TODO: this also needs to cancel probeHandler goroutine
						return
					}
				}
			}(destination, doneChan)
		}
		time.Sleep(REFRESH_INTERVAL * time.Minute)
	}
}
