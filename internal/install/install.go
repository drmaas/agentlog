package install

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drmaas/agentlog/pkg/agentlog"
)

//go:embed templates/*
var templateFS embed.FS

type Options struct {
	Global    bool
	SkillOnly bool
	HookOnly  bool
}

func Run(opts Options, cwd string) error {
	root, isGit := agentlog.FindRepoRoot(cwd)
	skillRoot := filepath.Join(root, ".agentlog", "skill")
	if opts.Global {
		home, _ := os.UserHomeDir()
		skillRoot = filepath.Join(home, ".agentlog", "skill")
	}

	if !opts.HookOnly {
		if err := installSkill(skillRoot); err != nil {
			return err
		}
	}
	if !opts.SkillOnly {
		if !opts.Global {
			if err := agentlog.WriteDefaultConfig(root); err != nil {
				return err
			}
			if isGit {
				if err := installHook(root); err != nil {
					return err
				}
			}
			if err := ensureGitignore(root); err != nil {
				return err
			}
		}
	}
	printMCPSnippet()
	return nil
}

func installSkill(skillRoot string) error {
	if err := os.MkdirAll(filepath.Join(skillRoot, "references"), 0o755); err != nil {
		return err
	}
	skill, err := templateFS.ReadFile("templates/skill.md")
	if err != nil {
		return err
	}
	exchange, err := templateFS.ReadFile("templates/exchange-format.md")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), skill, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillRoot, "references", "EXCHANGE-FORMAT.md"), exchange, 0o644)
}

func installHook(root string) error {
	hookSrc := filepath.Join(root, "hooks", "prepare-commit-msg")
	if _, err := os.Stat(hookSrc); err != nil {
		return nil
	}
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(hookSrc)
	if err != nil {
		return err
	}
	dst := filepath.Join(hooksDir, "prepare-commit-msg")
	return os.WriteFile(dst, data, 0o755)
}

func ensureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	needle := ".agentlog/sessions/"
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(data), needle) {
		return nil
	}
	out := strings.TrimRight(string(data), "\n")
	if out != "" {
		out += "\n"
	}
	out += needle + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

func printMCPSnippet() {
	fmt.Println("✓ Skill installed: AgentLog can now log agent exchanges")
	fmt.Println()
	fmt.Println("The skill instructs agents to call: agentlog log --request <...> --summary <...>")
	fmt.Println("Each call records the agent's intent and changes to the commit log.")
	fmt.Println()
	fmt.Println("Optional: For agents that prefer direct tool calls (MCP),")
	fmt.Println("add this to your agent's MCP configuration:")
	fmt.Println()
	fmt.Println("{")
	fmt.Println(`  "mcpServers": {`)
	fmt.Println(`    "agentlog": {`)
	fmt.Println(`      "command": "agentlog",`)
	fmt.Println(`      "args": ["mcp-serve"]`)
	fmt.Println("    }")
	fmt.Println("  }")
	fmt.Println("}")
}
