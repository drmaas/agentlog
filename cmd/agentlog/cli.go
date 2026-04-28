package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drmaas/agentlog/internal/install"
	"github.com/drmaas/agentlog/internal/mcp"
	"github.com/drmaas/agentlog/pkg/agentlog"
)

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "install":
		return runInstall(args[1:])
	case "start":
		return runStart(ctx, args[1:])
	case "log":
		return runLog(ctx, args[1:])
	case "end":
		return runEnd(ctx, args[1:])
	case "status":
		return runStatus(ctx)
	case "query":
		return runQuery(ctx, args[1:])
	case "show":
		return runShow(ctx, args[1:])
	case "mcp-serve":
		cwd, _ := os.Getwd()
		return mcp.Serve(ctx, cwd)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	global := fs.Bool("global", false, "install globally")
	skillOnly := fs.Bool("skill-only", false, "only install skill")
	hookOnly := fs.Bool("hook-only", false, "only install hook")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	return install.Run(install.Options{
		Global:    *global,
		SkillOnly: *skillOnly,
		HookOnly:  *hookOnly,
	}, cwd)
}

func runStart(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	user := fs.String("user", "", "user id")
	tags := fs.String("tags", "", "comma-separated tags")
	if err := fs.Parse(args); err != nil {
		return err
	}
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	session, err := manager.Start(ctx, agentlog.StartOpts{
		UserID: *user,
		Tags:   splitCSV(*tags),
	})
	if err != nil {
		return err
	}
	fmt.Println(session.SessionID)
	return nil
}

func runLog(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	sessionID := fs.String("session", "", "session id (optional if active session exists)")
	request := fs.String("request", "", "request text")
	summary := fs.String("summary", "", "response summary")
	files := fs.String("files", "", "comma-separated changed files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	ex, err := manager.Log(ctx, agentlog.LogOpts{
		SessionID:       *sessionID,
		Request:         *request,
		ResponseSummary: *summary,
		FilesChanged:    splitCSV(*files),
	})
	if err != nil {
		return err
	}
	fmt.Printf("logged exchange %s\n", ex.ExchangeID)
	return nil
}

func runEnd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("end", flag.ContinueOnError)
	sessionID := fs.String("session", "", "session id (optional if active session exists)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	s, links, err := manager.End(ctx, *sessionID)
	if err != nil {
		return err
	}
	fmt.Printf("ended %s (%d commit links)\n", s.SessionID, len(links))
	return nil
}

func runStatus(ctx context.Context) error {
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	id, s, err := manager.Status(ctx)
	if err != nil {
		return err
	}
	if id == "" || s == nil {
		fmt.Println("no active session")
		return nil
	}
	fmt.Printf("active session: %s user=%s branch=%s started=%s\n", s.SessionID, s.UserID, s.Branch, s.StartedAt.UTC().Format(time.RFC3339))
	return nil
}

func runQuery(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	sha := fs.String("sha", "", "commit SHA")
	file := fs.String("file", "", "file path")
	text := fs.String("text", "", "text query")
	user := fs.String("user", "", "user id")
	branch := fs.String("branch", "", "branch")
	since := fs.String("since", "", "RFC3339 timestamp")
	until := fs.String("until", "", "RFC3339 timestamp")
	limit := fs.Int("limit", 50, "result limit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	var filter agentlog.SessionFilter
	filter.UserID = *user
	filter.Branch = *branch
	filter.Limit = *limit
	if *since != "" {
		t, err := time.Parse(time.RFC3339, *since)
		if err != nil {
			return err
		}
		filter.Since = &t
	}
	if *until != "" {
		t, err := time.Parse(time.RFC3339, *until)
		if err != nil {
			return err
		}
		filter.Until = &t
	}
	sessions, exchanges, err := manager.Query(ctx, filter, *sha, *file, *text)
	if err != nil {
		return err
	}
	if exchanges != nil {
		for _, ex := range exchanges {
			fmt.Printf("%s [%d] %s\n", ex.ExchangeID, ex.Index, trimText(ex.Request, 100))
		}
		return nil
	}
	for _, s := range sessions {
		fmt.Printf("%s %s %s %s->%s\n", s.SessionID, s.StartedAt.UTC().Format(time.RFC3339), s.UserID, short(s.GitSHABefore), short(s.GitSHAAfter))
	}
	return nil
}

func runShow(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: agentlog show <session-id>")
	}
	manager, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer manager.Close()
	s, ex, links, err := manager.Show(ctx, args[0])
	if err != nil {
		return err
	}
	fmt.Printf("session: %s\nuser: %s\nstarted: %s\nended: %v\nbefore: %s\nafter: %s\n\n",
		s.SessionID, s.UserID, s.StartedAt.UTC().Format(time.RFC3339), s.EndedAt, s.GitSHABefore, s.GitSHAAfter)
	fmt.Printf("commits (%d):\n", len(links))
	for _, link := range links {
		fmt.Printf("- %s [%s]\n", link.CommitSHA, strings.Join(link.FilesChanged, ", "))
	}
	fmt.Printf("\nexchanges (%d):\n", len(ex))
	for _, e := range ex {
		fmt.Printf("- [%d] %s\n  req: %s\n  summary: %s\n", e.Index, e.Timestamp.UTC().Format(time.RFC3339), trimText(e.Request, 120), trimText(e.ResponseSummary, 120))
	}
	return nil
}

func newManager(ctx context.Context) (*agentlog.Manager, error) {
	cwd, _ := os.Getwd()
	root, _ := agentlog.FindRepoRoot(cwd)
	_ = os.MkdirAll(filepath.Join(root, ".agentlog", "sessions"), 0o755)
	_ = agentlog.WriteDefaultConfig(root)
	return agentlog.NewManager(ctx, root)
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	raw := strings.Split(v, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func trimText(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	return v[:n] + "..."
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func printUsage() {
	fmt.Println("agentlog commands:")
	fmt.Println("  install [--global] [--skill-only] [--hook-only]")
	fmt.Println("  start [--user USER] [--tags a,b]")
	fmt.Println("  log [--session ID] --request TEXT --summary TEXT [--files a,b]")
	fmt.Println("  end [--session ID]")
	fmt.Println("  status")
	fmt.Println("  query [--sha SHA | --file PATH | --text QUERY | --user USER --branch BRANCH --since RFC3339 --until RFC3339]")
	fmt.Println("  show <session-id>")
	fmt.Println("  mcp-serve")
}
