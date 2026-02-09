package store

import (
	"errors"
	"testing"
	"time"
)

func TestIsTransientSQLiteErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"non-transient", errors.New("syntax error"), false},
		{"SQLITE_BUSY text", errors.New("SQLITE_BUSY"), true},
		{"SQLITE_LOCKED text", errors.New("SQLITE_LOCKED"), true},
		{"IOERR_SHORT_READ text", errors.New("IOERR_SHORT_READ"), true},
		{"database is locked", errors.New("database is locked"), true},
		{"database table is locked", errors.New("database table is locked"), true},
		{"code 5", errors.New("sqlite: (5) database is busy"), true},
		{"code 6", errors.New("sqlite: (6) table is locked"), true},
		{"code 522", errors.New("sqlite: (522) short read"), true},
		{"wrapped busy", errors.New("exec: SQLITE_BUSY: db locked"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientSQLiteErr(tt.err)
			if got != tt.want {
				t.Errorf("isTransientSQLiteErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryOpSucceedsImmediately(t *testing.T) {
	calls := 0
	err := retryOp(defaultRetryConfig, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryOpNonTransientErrorNoRetry(t *testing.T) {
	calls := 0
	permanentErr := errors.New("syntax error near SELECT")
	err := retryOp(defaultRetryConfig, func() error {
		calls++
		return permanentErr
	})
	if err != permanentErr {
		t.Errorf("expected permanentErr, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for non-transient), got %d", calls)
	}
}

func TestRetryOpRetriesOnTransientError(t *testing.T) {
	calls := 0
	err := retryOp(retryConfig{maxRetries: 3, baseDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}, func() error {
		calls++
		if calls < 3 {
			return errors.New("SQLITE_BUSY")
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOpExhaustsRetries(t *testing.T) {
	calls := 0
	cfg := retryConfig{maxRetries: 2, baseDelay: time.Millisecond, maxDelay: 5 * time.Millisecond}
	err := retryOp(cfg, func() error {
		calls++
		return errors.New("SQLITE_BUSY")
	})
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	// maxRetries=2 means initial attempt + 2 retries = 3 total calls.
	if calls != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", calls)
	}
}

func TestRetryOpIOERRShortRead(t *testing.T) {
	calls := 0
	cfg := retryConfig{maxRetries: 2, baseDelay: time.Millisecond, maxDelay: 5 * time.Millisecond}
	err := retryOp(cfg, func() error {
		calls++
		if calls < 2 {
			return errors.New("(522) IOERR_SHORT_READ")
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestBackoffDelay(t *testing.T) {
	cfg := retryConfig{baseDelay: 50 * time.Millisecond, maxDelay: 500 * time.Millisecond}

	// Attempt 0: ~50ms + jitter
	d0 := backoffDelay(cfg, 0)
	if d0 < 50*time.Millisecond || d0 >= 100*time.Millisecond {
		t.Errorf("attempt 0 delay %v not in [50ms, 100ms)", d0)
	}

	// Attempt 1: ~100ms + jitter
	d1 := backoffDelay(cfg, 1)
	if d1 < 100*time.Millisecond || d1 >= 150*time.Millisecond {
		t.Errorf("attempt 1 delay %v not in [100ms, 150ms)", d1)
	}

	// Attempt 2: ~200ms + jitter
	d2 := backoffDelay(cfg, 2)
	if d2 < 200*time.Millisecond || d2 >= 250*time.Millisecond {
		t.Errorf("attempt 2 delay %v not in [200ms, 250ms)", d2)
	}
}

func TestBackoffDelayCapsAtMax(t *testing.T) {
	cfg := retryConfig{baseDelay: 100 * time.Millisecond, maxDelay: 200 * time.Millisecond}

	// Attempt 5: 100ms * 2^5 = 3200ms, should cap at 200ms + jitter
	d := backoffDelay(cfg, 5)
	if d >= 300*time.Millisecond {
		t.Errorf("attempt 5 delay %v should be capped near 200ms, got too high", d)
	}
}

func TestRetryOpZeroRetriesMeansOneAttempt(t *testing.T) {
	calls := 0
	cfg := retryConfig{maxRetries: 0, baseDelay: time.Millisecond, maxDelay: time.Millisecond}
	err := retryOp(cfg, func() error {
		calls++
		return errors.New("SQLITE_BUSY")
	})
	if err == nil {
		t.Error("expected error with 0 retries")
	}
	if calls != 1 {
		t.Errorf("expected 1 call with maxRetries=0, got %d", calls)
	}
}
