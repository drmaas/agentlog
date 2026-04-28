package markdown

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/drmaas/agentlog/pkg/agentlog"
)

func parseSessionMarkdown(md string) (*record, error) {
	lines := strings.Split(md, "\n")
	rec := &record{
		Session:    &agentlog.Session{Metadata: map[string]string{}},
		Exchanges:  []*agentlog.Exchange{},
		CommitLink: []*agentlog.CommitLink{},
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "| Session ID |") {
			rec.Session.SessionID = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| User |") {
			rec.Session.UserID = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| Started |") {
			t, err := time.Parse(timeFmt, extractBacktick(line))
			if err == nil {
				rec.Session.StartedAt = t
			}
		}
		if strings.HasPrefix(line, "| Ended |") {
			v := extractBacktick(line)
			if v != "" {
				t, err := time.Parse(timeFmt, v)
				if err == nil {
					rec.Session.EndedAt = &t
				}
			}
		}
		if strings.HasPrefix(line, "| Git SHA Before |") {
			rec.Session.GitSHABefore = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| Git SHA After |") {
			rec.Session.GitSHAAfter = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| Repository |") {
			rec.Session.Repository = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| Branch |") {
			rec.Session.Branch = extractBacktick(line)
		}
		if strings.HasPrefix(line, "| Tags |") {
			v := extractBacktick(line)
			if v != "" {
				rec.Session.Tags = strings.Split(v, ", ")
			}
		}
		if strings.HasPrefix(line, "- `") && strings.Contains(line, " — ") {
			parts := strings.SplitN(line, " — ", 2)
			sha := strings.Trim(parts[0], "- `")
			files := strings.Split(strings.Trim(parts[1], "`"), "`, `")
			rec.CommitLink = append(rec.CommitLink, &agentlog.CommitLink{
				SessionID:    rec.Session.SessionID,
				CommitSHA:    sha,
				FilesChanged: files,
			})
		}
		if strings.HasPrefix(line, "### [") {
			ex, consumed := parseExchange(lines[i:])
			if ex != nil {
				rec.Exchanges = append(rec.Exchanges, ex)
			}
			i += consumed
		}
	}

	if rec.Session.SessionID == "" {
		return nil, errors.New("invalid markdown session file")
	}
	return rec, nil
}

func parseExchange(lines []string) (*agentlog.Exchange, int) {
	if len(lines) == 0 {
		return nil, 0
	}
	header := lines[0]
	left := strings.Index(header, "[")
	right := strings.Index(header, "]")
	dash := strings.Index(header, "—")
	if left < 0 || right < 0 || dash < 0 {
		return nil, 0
	}
	idx, _ := strconv.Atoi(strings.TrimSpace(header[left+1 : right]))
	ts, _ := time.Parse(timeFmt, strings.TrimSpace(header[dash+3:]))
	ex := &agentlog.Exchange{
		Index:     idx,
		Timestamp: ts,
	}

	i := 1
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "### [") {
			break
		}
		if line == "**Request:**" && i+1 < len(lines) {
			ex.Request = strings.TrimSpace(lines[i+1])
		}
		if line == "**What changed:**" && i+1 < len(lines) {
			ex.ResponseSummary = strings.TrimSpace(lines[i+1])
		}
		if strings.HasPrefix(line, "**Files:**") {
			filesRaw := strings.TrimPrefix(line, "**Files:** ")
			filesRaw = strings.Trim(filesRaw, "`")
			if filesRaw != "" {
				ex.FilesChanged = strings.Split(filesRaw, "`, `")
			}
		}
	}
	return ex, i - 1
}

func extractBacktick(line string) string {
	start := strings.Index(line, "`")
	end := strings.LastIndex(line, "`")
	if start < 0 || end <= start {
		return ""
	}
	return line[start+1 : end]
}
