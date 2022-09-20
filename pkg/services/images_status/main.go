// FIXME: golangci-lint
// nolint:revive
package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// InfiniteLoopWait is the sleep time in minutes for the main loop
const InfiniteLoopWait = 1 // time in minutes to sleep between loop runs
// TODO: add to config when config is injected

func main() {
	log.Info("Images-Status microservice started")

	for {
		log.Info("Sleeping...")
		time.Sleep(InfiniteLoopWait * time.Minute)
	}
}
