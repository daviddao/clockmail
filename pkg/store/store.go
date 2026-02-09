// Package store manages all SQLite persistence for clockmail.
//
// SQLite in WAL mode serves as the broadcast medium: instead of Lamport's
// all-to-all message passing, agents read and write a shared database.
// The database IS the communication channel.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/daviddao/clockmail/pkg/clock"
	"github.com/daviddao/clockmail/pkg/model"

	_ "modernc.org/sqlite"
)

// Store manages all SQLite operations with WAL mode for concurrent access.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database and initializes the schema.
func New(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(60000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error { return s.db.Close() }

// retryOnContention wraps retryOp from retry.go with the default config.
// All store write operations should use this to handle transient SQLite
// errors (BUSY, LOCKED, IOERR_SHORT_READ) under concurrent access.
func retryOnContention(fn func() error) error {
	return retryOp(defaultRetryConfig, fn)
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id         TEXT PRIMARY KEY,
		clock      INTEGER NOT NULL DEFAULT 0,
		epoch      INTEGER NOT NULL DEFAULT 0,
		round      INTEGER NOT NULL DEFAULT 0,
		registered TEXT NOT NULL,
		last_seen  TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS events (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id   TEXT NOT NULL REFERENCES agents(id),
		lamport_ts INTEGER NOT NULL,
		epoch      INTEGER NOT NULL DEFAULT 0,
		round      INTEGER NOT NULL DEFAULT 0,
		kind       TEXT NOT NULL,
		target     TEXT,
		body       TEXT,
		created_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_lamport ON events(lamport_ts);
	CREATE INDEX IF NOT EXISTS idx_events_agent ON events(agent_id, lamport_ts);
	CREATE INDEX IF NOT EXISTS idx_events_kind_target ON events(kind, target);
	CREATE INDEX IF NOT EXISTS idx_events_epoch_round ON events(epoch, round);

	CREATE TABLE IF NOT EXISTS locks (
		path       TEXT NOT NULL,
		agent_id   TEXT NOT NULL REFERENCES agents(id),
		lamport_ts INTEGER NOT NULL,
		epoch      INTEGER NOT NULL DEFAULT 0,
		exclusive  INTEGER NOT NULL DEFAULT 1,
		expires_at TEXT NOT NULL,
		PRIMARY KEY (path, agent_id)
	);
	CREATE INDEX IF NOT EXISTS idx_locks_agent ON locks(agent_id);

	CREATE TABLE IF NOT EXISTS cursors (
		agent_id   TEXT PRIMARY KEY REFERENCES agents(id),
		since_ts   INTEGER NOT NULL DEFAULT 0
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

// RegisterAgent creates or updates an agent. Idempotent via ON CONFLICT.
func (s *Store) RegisterAgent(id string) (*model.Agent, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	err := retryOnContention(func() error {
		_, err := s.db.Exec(
			`INSERT INTO agents (id, clock, epoch, round, registered, last_seen)
			 VALUES (?, 0, 0, 0, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET last_seen = excluded.last_seen`,
			id, now, now,
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetAgent(id)
}

// GetAgent retrieves an agent by ID.
func (s *Store) GetAgent(id string) (*model.Agent, error) {
	row := s.db.QueryRow(
		`SELECT id, clock, epoch, round, registered, last_seen FROM agents WHERE id = ?`, id,
	)
	return scanAgent(row)
}

// UpdateAgentClock persists the agent's current Lamport clock and position.
func (s *Store) UpdateAgentClock(id string, clk, epoch, round int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return retryOnContention(func() error {
		_, err := s.db.Exec(
			`UPDATE agents SET clock = ?, epoch = ?, round = ?, last_seen = ? WHERE id = ?`,
			clk, epoch, round, now, id,
		)
		return err
	})
}

// ListAgents returns all registered agents ordered by ID.
func (s *Store) ListAgents() ([]model.Agent, error) {
	rows, err := s.db.Query(
		`SELECT id, clock, epoch, round, registered, last_seen FROM agents ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		var a model.Agent
		var regStr, lsStr string
		if err := rows.Scan(&a.ID, &a.Clock, &a.Epoch, &a.Round, &regStr, &lsStr); err != nil {
			return nil, err
		}
		var parseErr error
		a.Registered, parseErr = time.Parse(time.RFC3339Nano, regStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parse registered time for agent %s: %w", a.ID, parseErr)
		}
		a.LastSeen, parseErr = time.Parse(time.RFC3339Nano, lsStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parse last_seen time for agent %s: %w", a.ID, parseErr)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func scanAgent(row *sql.Row) (*model.Agent, error) {
	var a model.Agent
	var regStr, lsStr string
	if err := row.Scan(&a.ID, &a.Clock, &a.Epoch, &a.Round, &regStr, &lsStr); err != nil {
		return nil, err
	}
	var parseErr error
	a.Registered, parseErr = time.Parse(time.RFC3339Nano, regStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parse registered time for agent %s: %w", a.ID, parseErr)
	}
	a.LastSeen, parseErr = time.Parse(time.RFC3339Nano, lsStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parse last_seen time for agent %s: %w", a.ID, parseErr)
	}
	return &a, nil
}

// ---------------------------------------------------------------------------
// Cursors
// ---------------------------------------------------------------------------

// GetCursor returns the stored recv cursor for an agent (0 if unset).
func (s *Store) GetCursor(agentID string) int64 {
	var ts int64
	if err := s.db.QueryRow(
		`SELECT since_ts FROM cursors WHERE agent_id = ?`, agentID,
	).Scan(&ts); err != nil {
		return 0
	}
	return ts
}

// SetCursor updates the recv cursor for an agent.
func (s *Store) SetCursor(agentID string, sinceTS int64) error {
	return retryOnContention(func() error {
		_, err := s.db.Exec(
			`INSERT INTO cursors (agent_id, since_ts) VALUES (?, ?)
			 ON CONFLICT(agent_id) DO UPDATE SET since_ts = excluded.since_ts`,
			agentID, sinceTS,
		)
		return err
	})
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// InsertEvent appends an event to the log. Returns the auto-generated row ID.
func (s *Store) InsertEvent(e *model.Event) (int64, error) {
	var lastID int64
	err := retryOnContention(func() error {
		res, err := s.db.Exec(
			`INSERT INTO events (agent_id, lamport_ts, epoch, round, kind, target, body, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.AgentID, e.LamportTS, e.Epoch, e.Round, string(e.Kind), e.Target, e.Body,
			e.CreatedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
		lastID, err = res.LastInsertId()
		return err
	})
	return lastID, err
}

// ListEvents returns events with lamport_ts >= sinceTS, ordered by total order.
func (s *Store) ListEvents(sinceTS int64, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, agent_id, lamport_ts, epoch, round, kind,
		        COALESCE(target,''), COALESCE(body,''), created_at
		 FROM events WHERE lamport_ts >= ?
		 ORDER BY lamport_ts ASC, id ASC LIMIT ?`,
		sinceTS, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ListEventsSinceID returns events with row ID > sinceID, ordered by ID.
// This is useful for tailing the event log without missing events that
// share a Lamport timestamp.
func (s *Store) ListEventsSinceID(sinceID int64, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, agent_id, lamport_ts, epoch, round, kind,
		        COALESCE(target,''), COALESCE(body,''), created_at
		 FROM events WHERE id > ?
		 ORDER BY id ASC LIMIT ?`,
		sinceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// MaxEventID returns the highest event row ID, or 0 if the log is empty.
func (s *Store) MaxEventID() int64 {
	var id int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM events`).Scan(&id); err != nil {
		return 0
	}
	return id
}

// CountEvents returns the total number of events in the log.
// Unlike MaxEventID, this is correct even if event IDs have gaps.
func (s *Store) CountEvents() int64 {
	var count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		return 0
	}
	return count
}

// ListEventsForAgent returns messages targeted to agentID since sinceTS.
func (s *Store) ListEventsForAgent(agentID string, sinceTS int64, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, agent_id, lamport_ts, epoch, round, kind,
		        COALESCE(target,''), COALESCE(body,''), created_at
		 FROM events WHERE target = ? AND kind = 'msg' AND lamport_ts >= ?
		 ORDER BY lamport_ts ASC, id ASC LIMIT ?`,
		agentID, sinceTS, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]model.Event, error) {
	var events []model.Event
	for rows.Next() {
		var e model.Event
		var kindStr, createdStr string
		if err := rows.Scan(&e.ID, &e.AgentID, &e.LamportTS, &e.Epoch, &e.Round,
			&kindStr, &e.Target, &e.Body, &createdStr); err != nil {
			return nil, err
		}
		e.Kind = model.EventKind(kindStr)
		var parseErr error
		e.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parse created_at time for event %d: %w", e.ID, parseErr)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ---------------------------------------------------------------------------
// Locks
// ---------------------------------------------------------------------------

// AcquireLock attempts to acquire a file lock. Uses Lamport total order for
// deterministic conflict resolution: the agent with the lower (lamport_ts,
// agent_id) wins. Returns (granted_lock, nil, nil) on success, or
// (nil, conflicting_lock, nil) if another agent holds priority.
//
// The entire check-and-grant sequence runs inside a transaction to prevent
// TOCTOU races when two agents request the same lock concurrently.
func (s *Store) AcquireLock(path, agentID string, lamportTS, epoch int64, exclusive bool, ttl time.Duration) (*model.Lock, *model.Lock, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	// Expire stale locks outside the transaction (best-effort cleanup).
	s.expireStaleLocks()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Check for conflicts using Lamport total order.
	var conflict model.Lock
	var conflictExpires string
	err = tx.QueryRow(
		`SELECT path, agent_id, lamport_ts, epoch, exclusive, expires_at
		 FROM locks WHERE path = ? AND agent_id != ? AND exclusive = 1`,
		path, agentID,
	).Scan(&conflict.Path, &conflict.AgentID, &conflict.LamportTS, &conflict.Epoch,
		&conflict.Exclusive, &conflictExpires)

	if err == nil {
		var parseErr error
		conflict.ExpiresAt, parseErr = time.Parse(time.RFC3339Nano, conflictExpires)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse lock expires_at for %s: %w", conflict.Path, parseErr)
		}
		if clock.TotalOrderLess(lamportTS, agentID, conflict.LamportTS, conflict.AgentID) {
			// Requester wins — evict the existing lock.
			if _, err := tx.Exec(`DELETE FROM locks WHERE path = ? AND agent_id = ?`,
				path, conflict.AgentID); err != nil {
				return nil, nil, fmt.Errorf("evict lock: %w", err)
			}
		} else {
			// Existing holder wins — return conflict.
			return nil, &conflict, nil
		}
	}

	// Grant the lock.
	lock := model.Lock{
		Path:      path,
		AgentID:   agentID,
		LamportTS: lamportTS,
		Epoch:     epoch,
		Exclusive: exclusive,
		ExpiresAt: expiresAt,
	}
	_, err = tx.Exec(
		`INSERT INTO locks (path, agent_id, lamport_ts, epoch, exclusive, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(path, agent_id) DO UPDATE SET
		   lamport_ts = excluded.lamport_ts,
		   epoch = excluded.epoch,
		   exclusive = excluded.exclusive,
		   expires_at = excluded.expires_at`,
		path, agentID, lamportTS, epoch, boolToInt(exclusive),
		expiresAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit lock: %w", err)
	}
	return &lock, nil, nil
}

// ReleaseLock releases a file lock held by an agent.
func (s *Store) ReleaseLock(path, agentID string) error {
	return retryOnContention(func() error {
		_, err := s.db.Exec(`DELETE FROM locks WHERE path = ? AND agent_id = ?`, path, agentID)
		return err
	})
}

// ListLocks returns all active (non-expired) locks.
func (s *Store) ListLocks() ([]model.Lock, error) {
	s.expireStaleLocks()
	rows, err := s.db.Query(
		`SELECT path, agent_id, lamport_ts, epoch, exclusive, expires_at
		 FROM locks ORDER BY lamport_ts ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLocks(rows)
}

// ListLocksForAgent returns active locks held by a specific agent.
func (s *Store) ListLocksForAgent(agentID string) ([]model.Lock, error) {
	s.expireStaleLocks()
	rows, err := s.db.Query(
		`SELECT path, agent_id, lamport_ts, epoch, exclusive, expires_at
		 FROM locks WHERE agent_id = ? ORDER BY lamport_ts ASC`, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLocks(rows)
}

// GetActivePointstamps returns a Pointstamp per agent representing their
// current working position — used for frontier computation. Only includes
// agents seen within the last 10 minutes (considered alive).
func (s *Store) GetActivePointstamps() ([]model.Pointstamp, error) {
	agents, err := s.ListAgents()
	if err != nil {
		return nil, err
	}
	var ps []model.Pointstamp
	for _, a := range agents {
		if time.Since(a.LastSeen) < 10*time.Minute {
			ps = append(ps, model.Pointstamp{
				Timestamp: model.Timestamp{Epoch: a.Epoch, Round: a.Round},
				AgentID:   a.ID,
			})
		}
	}
	return ps, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *Store) expireStaleLocks() {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.Exec(`DELETE FROM locks WHERE expires_at < ?`, now)
}

func scanLocks(rows *sql.Rows) ([]model.Lock, error) {
	var locks []model.Lock
	for rows.Next() {
		var l model.Lock
		var expStr string
		var excl int
		if err := rows.Scan(&l.Path, &l.AgentID, &l.LamportTS, &l.Epoch, &excl, &expStr); err != nil {
			return nil, err
		}
		l.Exclusive = excl != 0
		var parseErr error
		l.ExpiresAt, parseErr = time.Parse(time.RFC3339Nano, expStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parse lock expires_at for %s: %w", l.Path, parseErr)
		}
		locks = append(locks, l)
	}
	return locks, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
