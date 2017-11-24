package zendb

import (
	"time"
	"log"
)

// extend duration constants, include originals for convenience
const (
	SECOND = time.Second
	MINUTE = time.Minute
	HOUR = time.Hour
	DAY = 24 * HOUR
	WEEK = 7 * DAY
)

type Scheduler struct {
	*time.Ticker
	run func()()
	done chan bool
}

func NewScheduler(tickTime time.Duration, run func()()) (Scheduler){
	return Scheduler{time.NewTicker(tickTime), run, make(chan bool)}
}

func (s *Scheduler) Start() {
	log.Print("INFO: Starting scheduler...")
	//Execute task immediately, schedule subsequent runs
	s.run()
	for {
		select {
			case <-s.Ticker.C:
				log.Print("INFO: Executing task")
				s.run()
			case <-s.done:
				s.Ticker.Stop()
				log.Print("INFO: Stopping scheduler")
				return
		}
	}
}

func (s *Scheduler) Stop() {
	s.done <- true
}

