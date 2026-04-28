package agentlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Manager struct {
	root    string
	backend StorageBackend
	cfg     Config
}

type StartOpts struct {
	UserID string
	Tags   []string
}

type LogOpts struct {
	SessionID       string
	Request         string
	ResponseSummary string
	FilesChanged    []string
	Metadata        map[string]string
}

func NewManager(ctx context.Context, root string) (*Manager, error) {
	repoRoot, _ := FindRepoRoot(root)
	cfg, err := LoadConfig(repoRoot)
	if err != nil {
		return nil, err
	}
	var backend StorageBackend
	for _, b := range cfg.Backends {
		args := map[string]string{}
		for k, v := range b.Args {
			if k == "path" && !filepath.IsAbs(v) {
				args[k] = filepath.Join(repoRoot, v)
			} else {
				args[k] = v
			}
		}
		created, createErr := CreateBackend(ctx, b.Type, args)
		if createErr == nil {
			backend = created
			break
		}
	}
	if backend == nil {
		return nil, errors.New("no backend configured")
	}
	return &Manager{
		root:    repoRoot,
		backend: backend,
		cfg:     cfg,
	}, nil
}

func (m *Manager) Close() error {
	return m.backend.Close()
}

func (m *Manager) Start(ctx context.Context, opts StartOpts) (*Session, error) {
	if err := os.MkdirAll(filepath.Join(m.root, ".agentlog"), 0o755); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("sess-%d", now.UnixNano())
	user := opts.UserID
	if user == "" {
		user = m.cfg.UserID
	}
	s := &Session{
		SessionID:    id,
		UserID:       user,
		StartedAt:    now,
		GitSHABefore: GitHead(m.root),
		Repository:   GitRepository(m.root),
		Branch:       GitBranch(m.root),
		Tags:         opts.Tags,
		Metadata:     map[string]string{},
	}
	if err := m.backend.WriteSession(ctx, s); err != nil {
		return nil, err
	}
	if err := m.writeActiveSession(s.SessionID); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) Log(ctx context.Context, opts LogOpts) (*Exchange, error) {
	if opts.Request == "" {
		return nil, errors.New("request is required")
	}
	if opts.ResponseSummary == "" {
		return nil, errors.New("response summary is required")
	}
	sessionID := opts.SessionID
	if sessionID == "" {
		active, err := m.ActiveSessionID()
		if err != nil {
			return nil, err
		}
		sessionID = active
	}
	exchanges, err := m.backend.GetExchanges(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	index := len(exchanges)
	e := &Exchange{
		ExchangeID:      fmt.Sprintf("%s-%03d", sessionID, index),
		Index:           index,
		Timestamp:       time.Now().UTC(),
		Request:         opts.Request,
		ResponseSummary: opts.ResponseSummary,
		FilesChanged:    uniqueNonEmpty(opts.FilesChanged),
		Metadata:        opts.Metadata,
	}
	if err := m.backend.WriteExchange(ctx, sessionID, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (m *Manager) End(ctx context.Context, sessionID string) (*Session, []CommitLink, error) {
	if sessionID == "" {
		active, err := m.ActiveSessionID()
		if err != nil {
			return nil, nil, err
		}
		sessionID = active
	}
	s, err := m.backend.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	s.EndedAt = &now
	s.GitSHAAfter = GitHead(m.root)
	if err := m.backend.UpdateSession(ctx, s); err != nil {
		return nil, nil, err
	}

	var links []CommitLink
	for _, sha := range GitCommitsBetween(m.root, s.GitSHABefore, s.GitSHAAfter) {
		l := CommitLink{
			SessionID:    s.SessionID,
			CommitSHA:    sha,
			FilesChanged: GitFilesForCommit(m.root, sha),
			LinkedAt:     now,
		}
		if err := m.backend.WriteCommitLink(ctx, &l); err != nil {
			return nil, nil, err
		}
		links = append(links, l)
	}
	_ = m.clearActiveSession()
	return s, links, nil
}

func (m *Manager) Status(ctx context.Context) (string, *Session, error) {
	id, err := m.ActiveSessionID()
	if err != nil {
		return "", nil, nil
	}
	s, err := m.backend.GetSession(ctx, id)
	if err != nil {
		return id, nil, err
	}
	return id, s, nil
}

func (m *Manager) Query(ctx context.Context, filter SessionFilter, sha, file, text string) ([]*Session, []*Exchange, error) {
	switch {
	case sha != "":
		s, err := m.backend.QueryByCommitSHA(ctx, sha)
		return s, nil, err
	case file != "":
		s, err := m.backend.QueryByFilePath(ctx, file)
		return s, nil, err
	case text != "":
		ex, err := m.backend.QueryByText(ctx, text)
		return nil, ex, err
	case filter.Since != nil && filter.Until != nil:
		s, err := m.backend.QueryByTimeRange(ctx, *filter.Since, *filter.Until)
		return s, nil, err
	default:
		s, err := m.backend.ListSessions(ctx, filter)
		return s, nil, err
	}
}

func (m *Manager) Show(ctx context.Context, sessionID string) (*Session, []*Exchange, []*CommitLink, error) {
	s, err := m.backend.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	exchanges, err := m.backend.GetExchanges(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	links, err := m.backend.GetCommitLinks(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	return s, exchanges, links, nil
}

type activeSession struct {
	SessionID string `json:"session_id"`
}

func (m *Manager) activeSessionPath() string {
	return filepath.Join(m.root, ".agentlog", "active_session.json")
}

func (m *Manager) writeActiveSession(sessionID string) error {
	data, _ := json.MarshalIndent(activeSession{SessionID: sessionID}, "", "  ")
	return os.WriteFile(m.activeSessionPath(), data, 0o644)
}

func (m *Manager) clearActiveSession() error {
	if err := os.Remove(m.activeSessionPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *Manager) ActiveSessionID() (string, error) {
	data, err := os.ReadFile(m.activeSessionPath())
	if err != nil {
		return "", errors.New("no active session")
	}
	var active activeSession
	if err := json.Unmarshal(data, &active); err != nil {
		return "", err
	}
	if active.SessionID == "" {
		return "", errors.New("no active session")
	}
	return active.SessionID, nil
}

func uniqueNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || slices.Contains(out, v) {
			continue
		}
		out = append(out, v)
	}
	return out
}
