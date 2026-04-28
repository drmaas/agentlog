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
	home, _ := os.UserHomeDir()
	
	// Determine install roots
	var claudeRoot, agentlogRoot string
	repoRoot, isGit := agentlog.FindRepoRoot(cwd)
	
	if opts.Global {
		// Global: ~/.claude for skills, ~/.agentlog for config/sessions
		claudeRoot = filepath.Join(home, ".claude")
		agentlogRoot = filepath.Join(home, ".agentlog")
	} else {
		// Local: repo/.claude for skills, repo/.agentlog for config/sessions
		claudeRoot = filepath.Join(repoRoot, ".claude")
		agentlogRoot = filepath.Join(repoRoot, ".agentlog")
	}

	// Install skill to .claude (always unless --hook-only or --skill-only with --global)
	if !opts.HookOnly {
		if err := installSkill(claudeRoot); err != nil {
			return err
		}
	}

	// Setup .agentlog directory and config (local repos unless --skill-only or --global)
	if !opts.SkillOnly && !opts.Global && !opts.HookOnly {
		if err := setupAgentlogDir(agentlogRoot); err != nil {
			return err
		}
		if isGit {
			if err := installHook(repoRoot); err != nil {
				return err
			}
		}
		if err := ensureGitignore(repoRoot); err != nil {
			return err
		}
	}

	// Hook-only: install hook without .agentlog setup
	if opts.HookOnly {
		if isGit {
			if err := installHook(repoRoot); err != nil {
				return err
			}
		}
	}

	// For --global with --skill-only, just install skill
	// For --global without flags, just install global skill
	// (Global installs don't setup .agentlog unless explicitly --skill-only is NOT set)
	if opts.Global && !opts.SkillOnly {
		if err := setupAgentlogConfig(agentlogRoot); err != nil {
			return err
		}
	}

	printCompletionMessage(opts, claudeRoot, agentlogRoot)
	return nil
}

func installSkill(claudeRoot string) error {
	skillPath := filepath.Join(claudeRoot, "skills", "agentlog.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		return err
	}
	skill, err := templateFS.ReadFile("templates/skill.md")
	if err != nil {
		return err
	}
	return os.WriteFile(skillPath, skill, 0o644)
}

func setupAgentlogDir(agentlogRoot string) error {
	// Create directory structure
	for _, dir := range []string{"", "sessions", "skill", filepath.Join("skill", "references")} {
		if err := os.MkdirAll(filepath.Join(agentlogRoot, dir), 0o755); err != nil {
			return err
		}
	}

	// Write config
	if err := setupAgentlogConfig(agentlogRoot); err != nil {
		return err
	}

	// Write skill to .agentlog/skill/ as well (for reference)
	skill, err := templateFS.ReadFile("templates/skill.md")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(agentlogRoot, "skill", "SKILL.md"), skill, 0o644); err != nil {
		return err
	}

	exchange, err := templateFS.ReadFile("templates/exchange-format.md")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(agentlogRoot, "skill", "references", "EXCHANGE-FORMAT.md"), exchange, 0o644)
}

func setupAgentlogConfig(agentlogRoot string) error {
	if err := os.MkdirAll(agentlogRoot, 0o755); err != nil {
		return err
	}
	// Check if config already exists
	configPath := filepath.Join(agentlogRoot, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists
	}
	// Write default config
	return agentlog.WriteDefaultConfig(agentlogRoot)
}

func installHook(root string) error {
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}
	
	hookData, err := templateFS.ReadFile("templates/prepare-commit-msg")
	if err != nil {
		return err
	}
	
	dst := filepath.Join(hooksDir, "prepare-commit-msg")
	return os.WriteFile(dst, hookData, 0o755)
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

func printCompletionMessage(opts Options, claudeRoot, agentlogRoot string) {
	fmt.Println("✓ AgentLog installed successfully")
	fmt.Println()

	if opts.HookOnly {
		fmt.Println("Git hook installed: prepare-commit-msg hook tagged commits with session IDs")
	} else if opts.SkillOnly {
		fmt.Printf("✓ Skill installed:  %s/agentlog.md\n", claudeRoot)
		fmt.Println("  Agents can now call agentlog_start, agentlog_log, agentlog_end, etc.")
	} else if opts.Global {
		fmt.Printf("✓ Global skill:     %s/skills/agentlog.md\n", claudeRoot)
		fmt.Printf("  Config template:  %s/config.yaml\n", agentlogRoot)
		fmt.Println()
		fmt.Println("  The global skill is available to all projects.")
		fmt.Println("  Create project-specific configs in each repo: .agentlog/config.yaml")
	} else {
		fmt.Printf("✓ Local skill:      %s/skills/agentlog.md\n", claudeRoot)
		fmt.Printf("  Config:           %s/config.yaml\n", agentlogRoot)
		fmt.Printf("  Sessions folder:  %s/sessions/\n", agentlogRoot)
		fmt.Println()
		fmt.Println("  Agents can now log exchanges. Run: agentlog start; agentlog log; agentlog end")
	}
}
