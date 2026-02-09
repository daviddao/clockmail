// iface.go defines the StoreInterface for dependency injection and testing.
//
// The concrete *Store type satisfies this interface. Code that depends on
// the store (e.g., the cmd layer, the viewer's snapshot builder) can accept
// StoreInterface instead of *Store, enabling mock injection in tests.
package store

import (
	"time"

	"github.com/daviddao/clockmail/pkg/model"
)

// StoreInterface defines the full set of store operations.
// The concrete *Store type implements this interface.
type StoreInterface interface {
	// Close closes the database connection.
	Close() error

	// --- Agents ---

	// RegisterAgent creates or updates an agent. Idempotent.
	RegisterAgent(id string) (*model.Agent, error)

	// GetAgent retrieves an agent by ID.
	GetAgent(id string) (*model.Agent, error)

	// UpdateAgentClock persists the agent's Lamport clock and position.
	UpdateAgentClock(id string, clk, epoch, round int64) error

	// ListAgents returns all registered agents ordered by ID.
	ListAgents() ([]model.Agent, error)

	// --- Cursors ---

	// GetCursor returns the stored recv cursor for an agent (0 if unset).
	GetCursor(agentID string) int64

	// SetCursor updates the recv cursor for an agent.
	SetCursor(agentID string, sinceTS int64) error

	// --- Events ---

	// InsertEvent appends an event to the log. Returns the row ID.
	InsertEvent(e *model.Event) (int64, error)

	// ListEvents returns events with lamport_ts >= sinceTS.
	ListEvents(sinceTS int64, limit int) ([]model.Event, error)

	// ListEventsSinceID returns events with row ID > sinceID.
	ListEventsSinceID(sinceID int64, limit int) ([]model.Event, error)

	// MaxEventID returns the highest event row ID, or 0 if empty.
	MaxEventID() int64

	// CountEvents returns the total number of events in the log.
	CountEvents() int64

	// ListEventsForAgent returns messages targeted to agentID.
	ListEventsForAgent(agentID string, sinceTS int64, limit int) ([]model.Event, error)

	// --- Locks ---

	// AcquireLock attempts to acquire a file lock.
	AcquireLock(path, agentID string, lamportTS, epoch int64, exclusive bool, ttl time.Duration) (*model.Lock, *model.Lock, error)

	// ReleaseLock releases a file lock held by an agent.
	ReleaseLock(path, agentID string) error

	// ListLocks returns all active (non-expired) locks.
	ListLocks() ([]model.Lock, error)

	// ListLocksForAgent returns active locks held by a specific agent.
	ListLocksForAgent(agentID string) ([]model.Lock, error)

	// --- Frontier ---

	// GetActivePointstamps returns pointstamps for active agents.
	GetActivePointstamps() ([]model.Pointstamp, error)
}

// Compile-time check that *Store implements StoreInterface.
var _ StoreInterface = (*Store)(nil)
