package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type event struct {
	name string
	ts   time.Time
}

var (
	// A subset of all elapsed file events, stored in memory. All events in
	// 'events' should be persisted to disk, so data is not lost if this process
	// crashes. Occasionally, events from 'events' are cleared, to free memory,
	// and any readers of 'events' must go to disk to get events that are older.
	events []event

	// A mutex guarding 'events'. This would be better done by a goroutine, but
	// using a mutex seems simpler to me
	mu sync.RWMutex
)

func updateEvents(inotifyLines []string) {
	mu.Lock()
	defer mu.Unlock()
	for _, l := range inotifyLines {
		ts1, name, _ := strings.Split(l, " ")
		ts2, err := time.Parse(time.RFC3339, ts1)
		if err != nil {
			// continue
		}
		events = append(events, event{
			name: name,
			ts:   ts2,
		})
	}
}

func startInotifyAndWatch() error {
	/* start inotifywait */
	args := []string{"inotifywait",
		"--event", "create,delete,modify",
		"--format", "%T %f %e",
		"--timefmt", "%FT%TZ%:z", // equivalent to RFC3339, I believe
		"--monitor",
		"\\",
	}
	cmd := exec.Command(args[0], args[1:]...)

	// get command's stdout and stderr pipes, to copy them to byte buffers
	var stdoutPipe io.Reader
	var err error
	stdoutPipe, err = cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("could not get stdout from inotifywait: %s\n(cmd:%s)",
			err,
			strings.Join(args, " "))
	}
	if err = cmd.Start(); err != nil {
		log.Fatalf("could start command \"%s\": %s",
			strings.Join(args, " "),
			err)
	}

	for _ := range time.Tick(30 * time.Second) {
		lines := make([]string)
		for s := bufio.NewScanner(stdoutPipe); s.Scan(); {
			lines = append(lines, s.Text())
		}
		updateEvents(lines)
	}
}

// Write all events in ['start', 'end'] to 'es'. If 'end' is in the future, this
// function does not return until 'end' is passed. To watch for new events, set
// 'end' to the distant future.
func getEvents(start, end time.Time, es <-chan event) {

}

func main() {
	net.ServeContent()
}
