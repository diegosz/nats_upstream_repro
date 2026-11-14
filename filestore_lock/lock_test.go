// Package filestore_lock checks whether an embedded nats-server with
// JetStream file storage releases its store directory after
// Shutdown() + WaitForShutdown(). Seen on Windows (GitHub windows-latest):
// a second server on the same StoreDir fails with "Access is denied" and the
// directory cannot be removed for a while. Linux is the control.
package filestore_lock

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"context"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const retryFor = 60 * time.Second

// capLogger records Errorf/Fatalf lines; Fatalf does not exit.
type capLogger struct {
	mu    sync.Mutex
	fatal []string
	errs  []string
}

func (l *capLogger) Noticef(string, ...any) {}
func (l *capLogger) Warnf(string, ...any)   {}
func (l *capLogger) Debugf(string, ...any)  {}
func (l *capLogger) Tracef(string, ...any)  {}
func (l *capLogger) Errorf(f string, v ...any) {
	l.mu.Lock()
	l.errs = append(l.errs, fmt.Sprintf(f, v...))
	l.mu.Unlock()
}
func (l *capLogger) Fatalf(f string, v ...any) {
	l.mu.Lock()
	l.fatal = append(l.fatal, fmt.Sprintf(f, v...))
	l.mu.Unlock()
}

// start returns a running JetStream server on dir, or the start error text.
func start(dir string) (*server.Server, error) {
	s, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoSigs: true,
		JetStream: true, StoreDir: dir,
	})
	if err != nil {
		return nil, err
	}
	lg := &capLogger{}
	s.SetLoggerV2(lg, false, false, false)
	s.Start()
	ready := s.ReadyForConnections(10 * time.Second)
	lg.mu.Lock()
	msgs := append(append([]string{}, lg.fatal...), lg.errs...)
	lg.mu.Unlock()
	if !ready || !s.JetStreamEnabled() || len(lg.fatal) > 0 {
		s.Shutdown()
		s.WaitForShutdown()
		return nil, fmt.Errorf("start failed (ready=%v js=%v): %s", ready, s.JetStreamEnabled(), strings.Join(msgs, " | "))
	}
	return s, nil
}

// populate creates a file-storage stream and writes msgs to it.
func populate(t *testing.T, s *server.Server, msgs int) {
	t.Helper()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name: "REPRO", Subjects: []string{"repro.>"}, Storage: jetstream.FileStorage,
	}); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 1024)
	for i := range msgs {
		if _, err := js.Publish(ctx, fmt.Sprintf("repro.%d", i%16), payload); err != nil {
			t.Fatal(err)
		}
	}
	// a KV bucket too: more files, as a typical app would have
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "repro", Storage: jetstream.FileStorage})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		if _, err := kv.Put(ctx, fmt.Sprintf("k%d", i), payload); err != nil {
			t.Fatal(err)
		}
	}
}

func shutdown(s *server.Server) time.Duration {
	t0 := time.Now()
	s.Shutdown()
	s.WaitForShutdown()
	return time.Since(t0)
}

func header(t *testing.T) {
	t.Logf("GOOS=%s GOARCH=%s go=%s nats-server=%s", runtime.GOOS, runtime.GOARCH, runtime.Version(), server.VERSION)
}

// TestRestartSameStoreDir: shut down, then immediately start a new server on
// the same StoreDir. Fails if the first attempt does not succeed; logs how
// long retries take until one does.
func TestRestartSameStoreDir(t *testing.T) {
	header(t)
	dir, err := os.MkdirTemp("", "nats-fs-lock-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for round := 1; round <= 5; round++ {
		s, err := start(dir)
		if err != nil {
			t.Fatalf("round %d: initial start: %v", round, err)
		}
		if round == 1 {
			populate(t, s, 2000)
		}
		t.Logf("round %d: Shutdown+WaitForShutdown took %v", round, shutdown(s))

		t0 := time.Now()
		var firstErr error
		attempts := 0
		for {
			attempts++
			s2, err := start(dir)
			if err == nil {
				shutdown(s2)
				break
			}
			if firstErr == nil {
				firstErr = err
			}
			if time.Since(t0) > retryFor {
				t.Fatalf("round %d: still failing after %v (%d attempts): %v", round, retryFor, attempts, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
		if firstErr != nil {
			t.Errorf("round %d: restart on same StoreDir failed right after WaitForShutdown: %v; succeeded after %v (%d attempts)",
				round, firstErr, time.Since(t0).Round(time.Millisecond), attempts)
		} else {
			t.Logf("round %d: restart on same StoreDir succeeded on first attempt", round)
		}
	}
}

// TestRemoveStoreDir: shut down, then immediately os.RemoveAll the StoreDir
// (what t.TempDir cleanup does). Fails if the first attempt does not succeed.
func TestRemoveStoreDir(t *testing.T) {
	header(t)
	dir, err := os.MkdirTemp("", "nats-fs-rm-*")
	if err != nil {
		t.Fatal(err)
	}
	s, err := start(dir)
	if err != nil {
		t.Fatal(err)
	}
	populate(t, s, 2000)
	t.Logf("Shutdown+WaitForShutdown took %v", shutdown(s))

	t0 := time.Now()
	var firstErr error
	attempts := 0
	for {
		attempts++
		err := os.RemoveAll(dir)
		if err == nil {
			if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
				break
			}
			err = fmt.Errorf("RemoveAll returned nil but %s still exists", dir)
		}
		if firstErr == nil {
			firstErr = err
			listOpen(t, dir)
		}
		if time.Since(t0) > retryFor {
			t.Fatalf("still cannot remove after %v (%d attempts): %v", retryFor, attempts, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if firstErr != nil {
		t.Errorf("RemoveAll right after WaitForShutdown failed: %v; succeeded after %v (%d attempts)",
			firstErr, time.Since(t0).Round(time.Millisecond), attempts)
	} else {
		t.Logf("RemoveAll succeeded on first attempt")
	}
}

// listOpen logs which files remain, to show what is still held.
func listOpen(t *testing.T, dir string) {
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && n < 40 {
			rel, _ := filepath.Rel(dir, p)
			t.Logf("  remaining: %s", rel)
			n++
		}
		return nil
	})
}
