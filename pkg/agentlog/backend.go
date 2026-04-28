package agentlog

import (
	"context"
	"time"
)

type StorageBackend interface {
	WriteSession(ctx context.Context, session *Session) error
	UpdateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListSessions(ctx context.Context, filter SessionFilter) ([]*Session, error)

	WriteExchange(ctx context.Context, sessionID string, exchange *Exchange) error
	GetExchanges(ctx context.Context, sessionID string) ([]*Exchange, error)

	WriteCommitLink(ctx context.Context, link *CommitLink) error
	GetCommitLinks(ctx context.Context, sessionID string) ([]*CommitLink, error)

	QueryByCommitSHA(ctx context.Context, sha string) ([]*Session, error)
	QueryByFilePath(ctx context.Context, path string) ([]*Session, error)
	QueryByTimeRange(ctx context.Context, start, end time.Time) ([]*Session, error)
	QueryByText(ctx context.Context, query string) ([]*Exchange, error)

	Init(ctx context.Context, config map[string]string) error
	Close() error
}
