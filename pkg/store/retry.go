// retry.go provides automatic retry logic for transient SQLite errors.
//
// Under high concurrency (4+ agents), WAL-mode SQLite can produce transient
// errors like SQLITE_BUSY, SQLITE_LOCKED, and IOERR_SHORT_READ (error 522).
// The busy_timeout pragma handles SQLITE_BUSY at the connection level, but
// other transient errors need application-level retries.
//
// This file provides a retryExec helper that wraps write operations with
// exponential backoff and jitter.
package store

import (
	"math/rand"
	"strings"
	"time"
)

// retryConfig controls retry behavior for transient SQLite errors.
type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// defaultRetryConfig is used for all store write operations.
var defaultRetryConfig = retryConfig{
	maxRetries: 3,
	baseDelay:  50 * time.Millisecond,
	maxDelay:   500 * time.Millisecond,
}

// isTransientSQLiteErr returns true if the error is a transient SQLite error
// that can be resolved by retrying. This includes:
//   - SQLITE_BUSY (5) — another connection holds a lock
//   - SQLITE_LOCKED (6) — table-level lock conflict
//   - SQLITE_IOERR_SHORT_READ (522) — WAL contention read failure
//   - database is locked — text-level detection for the busy_timeout fallthrough
func isTransientSQLiteErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite error codes embedded in error messages from modernc.org/sqlite.
	for _, pattern := range []string{
		"SQLITE_BUSY",
		"SQLITE_LOCKED",
		"IOERR_SHORT_READ",
		"database is locked",
		"database table is locked",
		"(5)",   // SQLITE_BUSY code
		"(6)",   // SQLITE_LOCKED code
		"(522)", // SQLITE_IOERR_SHORT_READ code
	} {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// retryOp executes fn with exponential backoff + jitter for transient errors.
// If fn succeeds or returns a non-transient error, it returns immediately.
func retryOp(cfg retryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isTransientSQLiteErr(lastErr) {
			return lastErr
		}
		if attempt < cfg.maxRetries {
			delay := backoffDelay(cfg, attempt)
			time.Sleep(delay)
		}
	}
	return lastErr
}

// backoffDelay computes the delay for a given retry attempt using exponential
// backoff with jitter: delay = baseDelay * 2^attempt + random([0, baseDelay)).
func backoffDelay(cfg retryConfig, attempt int) time.Duration {
	delay := cfg.baseDelay << uint(attempt) // baseDelay * 2^attempt
	if delay > cfg.maxDelay {
		delay = cfg.maxDelay
	}
	// Add jitter: random value in [0, baseDelay).
	jitter := time.Duration(rand.Int63n(int64(cfg.baseDelay)))
	return delay + jitter
}
