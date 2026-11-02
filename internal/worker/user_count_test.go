package worker

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type counterFunc func(context.Context) (int64, error)

func (f counterFunc) Count(ctx context.Context) (int64, error) {
	count, err := f(ctx)
	return count, err
}

type logWriter struct{ entries chan string }

func (w logWriter) Write(data []byte) (int, error) {
	w.entries <- string(data)
	return len(data), nil
}

func waitForWorker(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func nextLog(t *testing.T, entries <-chan string) string {
	t.Helper()
	select {
	case entry := <-entries:
		return entry
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not log the query result")
		return ""
	}
}

func TestLogUserCountStopsWithoutWaitingForTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	counter := counterFunc(func(context.Context) (int64, error) {
		t.Error("unexpected count after cancellation")
		return 0, nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		LogUserCount(ctx, counter, slog.Default())
	}()
	waitForWorker(t, done)
}

func TestLogUserCountRecoversAfterQueryFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time, 2)
	entries := make(chan string, 2)
	logger := slog.New(slog.NewTextHandler(logWriter{entries: entries}, nil))
	calls := 0
	counter := counterFunc(func(queryCtx context.Context) (int64, error) {
		deadline, ok := queryCtx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Error("count query has no bounded deadline")
		}
		calls++
		if calls == 1 {
			return 0, errors.New("temporary database failure")
		}
		return 42, nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		logUserCount(ctx, counter, logger, ticks)
	}()
	ticks <- time.Now()
	if entry := nextLog(t, entries); !strings.Contains(entry, "count users failed") {
		t.Fatalf("expected query failure log, got %s", entry)
	}
	ticks <- time.Now()
	if entry := nextLog(t, entries); !strings.Contains(entry, "total=42") {
		t.Fatalf("expected successful count after failure, got %s", entry)
	}
	cancel()
	waitForWorker(t, done)
	if calls != 2 {
		t.Fatalf("got %d count calls, want 2", calls)
	}
}

func TestLogUserCountCancelsInFlightQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time, 1)
	started := make(chan struct{})
	entries := make(chan string, 1)
	logger := slog.New(slog.NewTextHandler(logWriter{entries: entries}, nil))
	counter := counterFunc(func(queryCtx context.Context) (int64, error) {
		close(started)
		<-queryCtx.Done()
		return 0, queryCtx.Err()
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		logUserCount(ctx, counter, logger, ticks)
	}()
	ticks <- time.Now()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start the count query")
	}
	cancel()
	waitForWorker(t, done)
	select {
	case entry := <-entries:
		t.Fatalf("normal shutdown should not log a query failure: %s", entry)
	default:
	}
}
