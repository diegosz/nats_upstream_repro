// Package micro_race reproduces a data race in nats.go micro.AddService:
// the connection-closed handler (installed first) calls Stop, which ranges
// s.verbSubs under s.m, while AddService's addInternalHandler is still writing
// s.verbSubs without s.m. Run with -race.
package micro_race

import (
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

func startServer(t *testing.T) *server.Server {
	t.Helper()
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("server not ready")
	}
	t.Cleanup(func() { s.Shutdown(); s.WaitForShutdown() })
	return s
}

func iterations() int {
	if v, err := strconv.Atoi(os.Getenv("ITER")); err == nil && v > 0 {
		return v
	}
	return 2000
}

// TestAddServiceCloseRace closes the connection at a random moment while
// micro.AddService is registering its $SRV verb subscriptions.
func TestAddServiceCloseRace(t *testing.T) {
	s := startServer(t)
	cfg := micro.Config{Name: "repro", Version: "0.0.1"}

	var panics atomic.Int64
	n := iterations()
	defer func() { t.Logf("iterations=%d recovered AddService panics=%d", n, panics.Load()) }()
	for i := range n {
		nc, err := nats.Connect(s.ClientURL())
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			// nats.go <= v1.54.0 also panics here (nil *Subscription stored
			// after Subscribe fails on the closed conn; fixed by nats.go #2154).
			// Recover it so the race detector can report the map race.
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			_, _ = micro.AddService(nc, cfg) // an error after close is expected
		}()
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(rand.IntN(200)) * time.Microsecond)
			nc.Close()
		}()
		wg.Wait()
		if i%500 == 0 {
			t.Logf("iteration %d", i)
		}
	}
}
