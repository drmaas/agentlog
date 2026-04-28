package agentlog

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func FindRepoRoot(start string) (string, bool) {
	root, err := runGit(start, "rev-parse", "--show-toplevel")
	if err != nil || root == "" {
		return start, false
	}
	return filepath.Clean(root), true
}

func GitHead(root string) string {
	v, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return v
}

func GitBranch(root string) string {
	v, err := runGit(root, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return v
}

func GitRepository(root string) string {
	v, err := runGit(root, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return v
}

func GitCommitsBetween(root, before, after string) []string {
	if before == "" || after == "" || before == after {
		return nil
	}
	v, err := runGit(root, "log", "--format=%H", before+".."+after)
	if err != nil || v == "" {
		return nil
	}
	lines := strings.Split(v, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func GitFilesForCommit(root, sha string) []string {
	if sha == "" {
		return nil
	}
	v, err := runGit(root, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil || v == "" {
		return nil
	}
	lines := strings.Split(v, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
