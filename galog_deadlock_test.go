package galog

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type blockingBackend struct {
	id        string
	config    Config
	blockCh   chan struct{}
	logCount  int32
	logCalled chan struct{}
}

func newBlockingBackend(id string) *blockingBackend {
	return &blockingBackend{
		id:        id,
		config:    newBackendConfig(10),
		blockCh:   make(chan struct{}),
		logCalled: make(chan struct{}),
	}
}

func (b *blockingBackend) ID() string {
	return b.id
}

func (b *blockingBackend) Log(entry *LogEntry) error {
	count := atomic.AddInt32(&b.logCount, 1)
	println(fmt.Sprintf("blockingBackend.Log called! count=%d", count))

	if count == 1 {
		// First call: return error to force enqueuing
		println("First log: returning error to force queueing.")
		return fmt.Errorf("simulated write failure")
	}

	// Second call (from flush): block forever
	println("Second log (flush): blocking forever.")
	close(b.logCalled)
	<-b.blockCh
	println("blockingBackend.Log unblocked!")
	return nil
}

func (b *blockingBackend) Config() Config {
	return b.config
}

func (b *blockingBackend) Shutdown(ctx context.Context) error {
	println("blockingBackend.Shutdown called!")
	return nil
}

func TestShutdownDeadlock(t *testing.T) {
	tl := newTestLogger()
	tl.SetLevel(DebugLevel)

	globalLogger := defaultLogger
	defaultLogger = tl
	t.Cleanup(func() {
		defaultLogger = globalLogger
	})

	backend := newBlockingBackend("blocking-backend")
	RegisterBackend(context.Background(), backend)

	// Log a message to trigger background writer goroutine
	Debugf("This log will fail first and then block on flush")

	// Wait for Log to be called and block on the second attempt
	select {
	case <-backend.logCalled:
		t.Log("Confirmed: Log was called twice and is now blocking on flush.")
	case <-time.After(2 * time.Second):
		t.Fatal("Fatal: Log did not block on flush within 2s!")
	}

	shutdownDone := make(chan struct{})
	go func() {
		println("Calling Shutdown...")
		// Call Shutdown with a short timeout
		Shutdown(100 * time.Millisecond)
		println("Shutdown returned!")
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Log("Shutdown completed successfully within timeout!")
	case <-time.After(2 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Shutdown did not complete within 2s (timeout parameter was 100ms)")
	}
}
