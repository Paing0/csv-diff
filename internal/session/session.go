package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"csv-diff/internal/csvparser"
	"csv-diff/internal/diff"
	"csv-diff/internal/model"
)

// ComparisonSession represents an active user session holding uploaded files and comparison results.
type ComparisonSession struct {
	ID           string
	CreatedAt    time.Time
	LastAccessed time.Time

	OriginalFile *csvparser.ParsedCSV
	ModifiedFile *csvparser.ParsedCSV
	OrigMeta     model.FileMetadata
	ModMeta      model.FileMetadata

	LastOptions *model.CompareOptions
	Summary     *model.CompareSummary
	DiffRows    []model.RowDiff

	CancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// Manager manages in-memory comparison sessions with thread safety and automatic expiration.
type Manager struct {
	sessions map[string]*ComparisonSession
	engine   *diff.Engine
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewManager creates a session manager with the given TTL for inactive sessions.
func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	m := &Manager{
		sessions: make(map[string]*ComparisonSession),
		engine:   diff.NewEngine(),
		ttl:      ttl,
	}

	// Background session cleanup routine
	go m.cleanupLoop(10 * time.Minute)

	return m
}

func (m *Manager) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		m.CleanupExpired()
	}
}

// CleanupExpired removes sessions that haven't been accessed within the TTL.
func (m *Manager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.ttl)
	for id, s := range m.sessions {
		s.mu.RLock()
		lastAccessed := s.LastAccessed
		s.mu.RUnlock()

		if lastAccessed.Before(cutoff) {
			s.mu.Lock()
			if s.CancelFunc != nil {
				s.CancelFunc()
			}
			s.mu.Unlock()
			delete(m.sessions, id)
		}
	}
}

// CreateSession initializes a new session with uploaded CSV files.
func (m *Manager) CreateSession(id string, orig, mod *csvparser.ParsedCSV, origMeta, modMeta model.FileMetadata) *ComparisonSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := &ComparisonSession{
		ID:           id,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		OriginalFile: orig,
		ModifiedFile: mod,
		OrigMeta:     origMeta,
		ModMeta:      modMeta,
	}
	m.sessions[id] = sess
	return sess
}

// GetSession retrieves an active session by ID.
func (m *Manager) GetSession(id string) (*ComparisonSession, error) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	sess.mu.Lock()
	sess.LastAccessed = time.Now()
	sess.mu.Unlock()

	return sess, nil
}

// RunComparison executes comparison and caches the results in the session.
func (m *Manager) RunComparison(ctx context.Context, sessionID string, opts model.CompareOptions) (*model.CompareSummary, []model.RowDiff, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, nil, err
	}

	sess.mu.Lock()
	// Cancel any previously running comparison
	if sess.CancelFunc != nil {
		sess.CancelFunc()
	}
	cmpCtx, cancel := context.WithCancel(ctx)
	sess.CancelFunc = cancel
	orig := sess.OriginalFile
	mod := sess.ModifiedFile
	sess.mu.Unlock()

	summary, diffRows, err := m.engine.Compare(cmpCtx, orig, mod, opts)
	if err != nil {
		return summary, nil, err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.LastOptions = &opts
	sess.Summary = summary
	sess.DiffRows = diffRows
	sess.CancelFunc = nil

	return summary, diffRows, nil
}

// CancelComparison cancels any active comparison running in the session.
func (m *Manager) CancelComparison(sessionID string) error {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.CancelFunc != nil {
		sess.CancelFunc()
		sess.CancelFunc = nil
		return nil
	}
	return nil
}

// QueryResults paginates and filters cached comparison results without re-running diffing.
func (m *Manager) QueryResults(sessionID string, req model.QueryResultsRequest) (*model.QueryResultsResponse, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	sess.mu.RLock()
	defer sess.mu.RUnlock()

	if sess.Summary == nil {
		return nil, fmt.Errorf("no comparison results available for session: %s", sessionID)
	}

	allDiffs := sess.DiffRows
	var filtered []model.RowDiff

	// Filter by status and search text
	filterStatus := strings.ToLower(strings.TrimSpace(req.Filter))
	searchQuery := strings.ToLower(strings.TrimSpace(req.Search))

	for _, row := range allDiffs {
		// Filter by status
		if filterStatus != "" && filterStatus != "all" {
			if string(row.Status) != filterStatus {
				continue
			}
		}

		// Filter by search query
		if searchQuery != "" {
			matched := false
			if strings.Contains(strings.ToLower(row.Key), searchQuery) {
				matched = true
			} else {
				for _, cell := range row.Cells {
					if cell.OriginalValue != nil && strings.Contains(strings.ToLower(*cell.OriginalValue), searchQuery) {
						matched = true
						break
					}
					if cell.ModifiedValue != nil && strings.Contains(strings.ToLower(*cell.ModifiedValue), searchQuery) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		filtered = append(filtered, row)
	}

	totalFiltered := len(filtered)
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > totalFiltered {
		offset = totalFiltered
	}

	limit := req.Limit
	var pagedRows []model.RowDiff
	hasMore := false

	if limit <= 0 {
		// Return all remaining from offset
		pagedRows = filtered[offset:]
		hasMore = false
	} else {
		end := offset + limit
		if end < totalFiltered {
			pagedRows = filtered[offset:end]
			hasMore = true
		} else {
			pagedRows = filtered[offset:]
			hasMore = false
		}
	}

	return &model.QueryResultsResponse{
		SessionID:     sessionID,
		TotalFiltered: totalFiltered,
		Offset:        offset,
		Limit:         limit,
		HasMore:       hasMore,
		Rows:          pagedRows,
	}, nil
}
