package markdown

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/drmaas/agentlog/pkg/agentlog"
)

func init() {
	agentlog.RegisterBackend("markdown", func() agentlog.StorageBackend {
		return &Backend{}
	})
}

type record struct {
	Session    *agentlog.Session      `json:"session"`
	Exchanges  []*agentlog.Exchange   `json:"exchanges"`
	CommitLink []*agentlog.CommitLink `json:"commit_links"`
}

type Backend struct {
	mu   sync.Mutex
	base string
}

func (b *Backend) Init(_ context.Context, config map[string]string) error {
	base := config["path"]
	if base == "" {
		base = ".agentlog/sessions"
	}
	b.base = base
	return os.MkdirAll(b.base, 0o755)
}

func (b *Backend) Close() error { return nil }

func (b *Backend) WriteSession(_ context.Context, session *agentlog.Session) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec := &record{Session: session, Exchanges: []*agentlog.Exchange{}, CommitLink: []*agentlog.CommitLink{}}
	return b.writeRecord(rec)
}

func (b *Backend) UpdateSession(_ context.Context, session *agentlog.Session) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(session.SessionID)
	if err != nil {
		return err
	}
	rec.Session = session
	return b.writeRecord(rec)
}

func (b *Backend) GetSession(_ context.Context, sessionID string) (*agentlog.Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	return rec.Session, nil
}

func (b *Backend) ListSessions(_ context.Context, filter agentlog.SessionFilter) ([]*agentlog.Session, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	sessions := make([]*agentlog.Session, 0, len(recs))
	for _, rec := range recs {
		if rec.Session == nil {
			continue
		}
		if filter.UserID != "" && rec.Session.UserID != filter.UserID {
			continue
		}
		if filter.Branch != "" && rec.Session.Branch != filter.Branch {
			continue
		}
		if filter.Since != nil && rec.Session.StartedAt.Before(*filter.Since) {
			continue
		}
		if filter.Until != nil && rec.Session.StartedAt.After(*filter.Until) {
			continue
		}
		sessions = append(sessions, rec.Session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	if filter.Offset > 0 && filter.Offset < len(sessions) {
		sessions = sessions[filter.Offset:]
	}
	if filter.Limit > 0 && len(sessions) > filter.Limit {
		sessions = sessions[:filter.Limit]
	}
	return sessions, nil
}

func (b *Backend) WriteExchange(_ context.Context, sessionID string, exchange *agentlog.Exchange) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(sessionID)
	if err != nil {
		return err
	}
	rec.Exchanges = append(rec.Exchanges, exchange)
	return b.writeRecord(rec)
}

func (b *Backend) GetExchanges(_ context.Context, sessionID string) ([]*agentlog.Exchange, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	return rec.Exchanges, nil
}

func (b *Backend) WriteCommitLink(_ context.Context, link *agentlog.CommitLink) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(link.SessionID)
	if err != nil {
		return err
	}
	rec.CommitLink = append(rec.CommitLink, link)
	return b.writeRecord(rec)
}

func (b *Backend) GetCommitLinks(_ context.Context, sessionID string) ([]*agentlog.CommitLink, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.readRecordBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	return rec.CommitLink, nil
}

func (b *Backend) QueryByCommitSHA(_ context.Context, sha string) ([]*agentlog.Session, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	var out []*agentlog.Session
	for _, rec := range recs {
		for _, link := range rec.CommitLink {
			if strings.Contains(link.CommitSHA, sha) {
				out = append(out, rec.Session)
				break
			}
		}
	}
	return dedupeSessions(out), nil
}

func (b *Backend) QueryByFilePath(_ context.Context, path string) ([]*agentlog.Session, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	var out []*agentlog.Session
	for _, rec := range recs {
		matched := false
		for _, ex := range rec.Exchanges {
			for _, f := range ex.FilesChanged {
				if strings.Contains(f, path) {
					out = append(out, rec.Session)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			continue
		}
		for _, cl := range rec.CommitLink {
			for _, f := range cl.FilesChanged {
				if strings.Contains(f, path) {
					out = append(out, rec.Session)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}
	return dedupeSessions(out), nil
}

func (b *Backend) QueryByTimeRange(_ context.Context, start, end time.Time) ([]*agentlog.Session, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	var out []*agentlog.Session
	for _, rec := range recs {
		if rec.Session.StartedAt.After(start) && rec.Session.StartedAt.Before(end) {
			out = append(out, rec.Session)
		}
	}
	return out, nil
}

func (b *Backend) QueryByText(_ context.Context, query string) ([]*agentlog.Exchange, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	var out []*agentlog.Exchange
	for _, rec := range recs {
		for _, ex := range rec.Exchanges {
			if strings.Contains(strings.ToLower(ex.Request), query) ||
				strings.Contains(strings.ToLower(ex.ResponseSummary), query) {
				out = append(out, ex)
			}
		}
	}
	return out, nil
}

func (b *Backend) readRecordBySessionID(sessionID string) (*record, error) {
	recs, err := b.loadAll()
	if err != nil {
		return nil, err
	}
	for _, rec := range recs {
		if rec.Session != nil && rec.Session.SessionID == sessionID {
			return rec, nil
		}
	}
	return nil, errors.New("session not found")
}

func (b *Backend) loadAll() ([]*record, error) {
	files, err := filepath.Glob(filepath.Join(b.base, "*.md"))
	if err != nil {
		return nil, err
	}
	recs := make([]*record, 0, len(files))
	for _, file := range files {
		if strings.HasSuffix(file, "index.md") {
			continue
		}
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}
		rec, parseErr := parseSessionMarkdown(string(data))
		if parseErr != nil {
			return nil, parseErr
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

func dedupeSessions(in []*agentlog.Session) []*agentlog.Session {
	seen := map[string]bool{}
	out := make([]*agentlog.Session, 0, len(in))
	for _, s := range in {
		if s == nil || seen[s.SessionID] {
			continue
		}
		seen[s.SessionID] = true
		out = append(out, s)
	}
	return out
}
