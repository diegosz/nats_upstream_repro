package filestore_lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func rounds() int {
	if v, err := strconv.Atoi(os.Getenv("ROUNDS")); err == nil && v > 0 {
		return v
	}
	return 50
}

// live is a client that keeps working against the server: a publisher
// goroutine and a durable pull consumer that acks everything.
type live struct {
	nc    *nats.Conn
	cc    jetstream.ConsumeContext
	stop  chan struct{}
	done  sync.WaitGroup
	pub   atomic.Int64
	acked atomic.Int64
}

func startLive(t *testing.T, s *server.Server) *live {
	t.Helper()
	// No reconnect: the next server listens on a new port anyway.
	nc, err := nats.Connect(s.ClientURL(), nats.NoReconnect(),
		nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
	if err != nil {
		t.Fatal(err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: "LIVE", Subjects: []string{"live.>"}, Storage: jetstream.FileStorage,
		MaxMsgs: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	cons, err := st.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: "worker", AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := st.Info(ctx); err == nil && os.Getenv("DEBUG") != "" {
		t.Logf("  stream state: msgs=%d bytes=%d last=%d", info.State.Msgs, info.State.Bytes, info.State.LastSeq)
	}
	l := &live{nc: nc, stop: make(chan struct{})}
	l.cc, err = cons.Consume(func(m jetstream.Msg) {
		if m.Ack() == nil {
			l.acked.Add(1)
		}
	}, jetstream.ConsumeErrHandler(func(jetstream.ConsumeContext, error) {}))
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 512)
	l.done.Add(1)
	go func() {
		defer l.done.Done()
		for i := 0; ; i++ {
			select {
			case <-l.stop:
				return
			default:
			}
			if _, err := js.PublishAsync(fmt.Sprintf("live.%d", i%8), payload); err != nil {
				if errors.Is(err, nats.ErrConnectionClosed) || errors.Is(err, nats.ErrConnectionDraining) {
					return
				}
				time.Sleep(time.Millisecond) // too many pending, or disconnected
				continue
			}
			l.pub.Add(1)
		}
	}()
	return l
}

func (l *live) close() {
	close(l.stop)
	l.done.Wait()
	l.cc.Stop()
	l.nc.Close()
}

// restartRetry starts a server on dir, retrying until it works.
// It returns the server, the first error (nil if the first attempt worked),
// the time until success and the attempts made.
func restartRetry(t *testing.T, dir string) (*server.Server, error, time.Duration, int) {
	t.Helper()
	t0 := time.Now()
	var firstErr error
	for attempts := 1; ; attempts++ {
		s, err := start(dir)
		if err == nil {
			return s, firstErr, time.Since(t0), attempts
		}
		if firstErr == nil {
			firstErr = err
		}
		if time.Since(t0) > retryFor {
			t.Fatalf("still failing after %v (%d attempts): %v", retryFor, attempts, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestRestartWithLiveClient: each round, the server is shut down while a
// client is connected with a publisher and a durable consumer running; then a
// new server starts on the same StoreDir right after WaitForShutdown. Finally
// the dir is removed right after a last shutdown, also with a live client.
func TestRestartWithLiveClient(t *testing.T) {
	header(t)
	dir, err := os.MkdirTemp("", "nats-fs-live-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	n := rounds()
	s, err := start(dir)
	if err != nil {
		t.Fatal(err)
	}
	var failed int
	var worst time.Duration
	for round := 1; round <= n; round++ {
		l := startLive(t, s)
		time.Sleep(300 * time.Millisecond) // let traffic flow and files grow
		sd := shutdown(s)                  // client still connected and busy

		s2, firstErr, took, attempts := restartRetry(t, dir)
		if firstErr != nil {
			failed++
			worst = max(worst, took)
			t.Logf("round %d: FAIL restart right after WaitForShutdown (%v): %v; ok after %v, %d attempts",
				round, sd, firstErr, took.Round(time.Millisecond), attempts)
		} else if round%10 == 0 || round == 1 {
			t.Logf("round %d: ok first attempt (shutdown %v, published %d, acked %d)",
				round, sd, l.pub.Load(), l.acked.Load())
		}
		l.close()
		s = s2
	}

	// Final: shutdown with a live client, then remove the dir at once.
	l := startLive(t, s)
	time.Sleep(300 * time.Millisecond)
	shutdown(s)
	t0 := time.Now()
	var rmFirst error
	for attempts := 1; ; attempts++ {
		err := os.RemoveAll(dir)
		if err == nil {
			if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
				break
			}
			err = fmt.Errorf("RemoveAll returned nil but %s still exists", dir)
		}
		if rmFirst == nil {
			rmFirst = err
			listOpen(t, dir)
		}
		if time.Since(t0) > retryFor {
			t.Fatalf("still cannot remove after %v: %v", retryFor, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	l.close()

	t.Logf("summary: %d/%d restarts failed on the first attempt (worst recovery %v)", failed, n, worst.Round(time.Millisecond))
	if failed > 0 {
		t.Errorf("%d/%d restarts on the same StoreDir failed right after WaitForShutdown", failed, n)
	}
	if rmFirst != nil {
		t.Errorf("RemoveAll right after WaitForShutdown failed: %v; ok after %v", rmFirst, time.Since(t0).Round(time.Millisecond))
	}
}
