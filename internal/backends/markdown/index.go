package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (b *Backend) writeIndex() error {
	recs, err := b.loadAll()
	if err != nil {
		return err
	}
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Session.StartedAt.After(recs[j].Session.StartedAt)
	})

	var out strings.Builder
	out.WriteString("# AgentLog Session Index\n\n")
	out.WriteString("| Started | User | Branch | Before SHA | After SHA | Session ID | File |\n")
	out.WriteString("|---|---|---|---|---|---|---|\n")

	for _, rec := range recs {
		s := rec.Session
		filename := sessionFilename(s)
		out.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s | %s | %s | [link](sessions/%s) |\n",
			s.StartedAt.UTC().Format("2006-01-02T15:04Z"),
			s.UserID,
			s.Branch,
			shortSHA(s.GitSHABefore),
			shortSHA(s.GitSHAAfter),
			s.SessionID,
			filename,
		))
	}
	indexPath := filepath.Join(filepath.Dir(b.base), "index.md")
	return os.WriteFile(indexPath, []byte(out.String()), 0o644)
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
