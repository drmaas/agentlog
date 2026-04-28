package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drmaas/agentlog/pkg/agentlog"
)

func (b *Backend) writeRecord(rec *record) error {
	filename := sessionFilename(rec.Session)
	fullPath := filepath.Join(b.base, filename)
	content := renderSessionMarkdown(rec)
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return err
	}
	return b.writeIndex()
}

func sessionFilename(s *agentlog.Session) string {
	ts := s.StartedAt.UTC().Format("2006-01-02T15-04-05Z")
	sha := s.GitSHABefore
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if sha == "" {
		sha = "nogit"
	}
	user := sanitizeFilename(s.UserID)
	return fmt.Sprintf("%s_%s_%s_%s.md", ts, user, sha, s.SessionID)
}

func sanitizeFilename(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	v = strings.ReplaceAll(v, " ", "_")
	v = strings.ReplaceAll(v, "/", "_")
	return v
}

func renderSessionMarkdown(rec *record) string {
	s := rec.Session
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Session: %s\n\n", s.SessionID))
	b.WriteString("| Field | Value |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| Session ID | `%s` |\n", s.SessionID))
	b.WriteString(fmt.Sprintf("| User | `%s` |\n", s.UserID))
	b.WriteString(fmt.Sprintf("| Started | `%s` |\n", s.StartedAt.UTC().Format(timeFmt)))
	if s.EndedAt != nil {
		b.WriteString(fmt.Sprintf("| Ended | `%s` |\n", s.EndedAt.UTC().Format(timeFmt)))
	} else {
		b.WriteString("| Ended | `` |\n")
	}
	b.WriteString(fmt.Sprintf("| Git SHA Before | `%s` |\n", s.GitSHABefore))
	b.WriteString(fmt.Sprintf("| Git SHA After | `%s` |\n", s.GitSHAAfter))
	b.WriteString(fmt.Sprintf("| Repository | `%s` |\n", s.Repository))
	b.WriteString(fmt.Sprintf("| Branch | `%s` |\n", s.Branch))
	if len(s.Tags) > 0 {
		b.WriteString(fmt.Sprintf("| Tags | `%s` |\n", strings.Join(s.Tags, ", ")))
	} else {
		b.WriteString("| Tags | `` |\n")
	}

	b.WriteString("\n## Commits\n\n")
	if len(rec.CommitLink) == 0 {
		b.WriteString("- _None_\n")
	} else {
		for _, c := range rec.CommitLink {
			b.WriteString(fmt.Sprintf("- `%s` — `%s`\n", c.CommitSHA, strings.Join(c.FilesChanged, "`, `")))
		}
	}

	b.WriteString("\n## Exchanges\n\n")
	if len(rec.Exchanges) == 0 {
		b.WriteString("_No exchanges recorded yet._\n")
	} else {
		for _, ex := range rec.Exchanges {
			b.WriteString(fmt.Sprintf("### [%d] — %s\n\n", ex.Index, ex.Timestamp.UTC().Format(timeFmt)))
			b.WriteString("**Request:**\n")
			b.WriteString(ex.Request + "\n\n")
			b.WriteString("**What changed:**\n")
			b.WriteString(ex.ResponseSummary + "\n\n")
			if len(ex.FilesChanged) > 0 {
				b.WriteString("**Files:** `" + strings.Join(ex.FilesChanged, "`, `") + "`\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

const timeFmt = "2006-01-02T15:04:05Z"
