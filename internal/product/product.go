// Package product runs the claude-obsidian CLI that ships inside its plugin.
package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/claudecode"
	"github.com/nathanaday/claude-atlas/internal/home"
)

// TestedVersion is the claude-obsidian release this atlas was verified against.
const TestedVersion = "2.2.0"

const cliScript = "scripts/claude-obsidian.py"

// Product is an installed claude-obsidian tree.
type Product struct {
	Root    string
	Version string
	// Source says where Root came from: "config" or "plugin".
	Source string
}

// Version reads .claude-plugin/plugin.json under root.
func versionAt(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return manifest.Version
}

// IsProduct reports whether root holds a runnable claude-obsidian.
func IsProduct(root string) bool {
	info, err := os.Stat(filepath.Join(root, cliScript))
	return err == nil && info.Mode().IsRegular()
}

var ErrNotInstalled = errors.New("claude-obsidian is not installed")

// Locate finds the product: the configured path, else the installed Claude Code plugin.
func Locate(cfg home.ProductConfig) (*Product, error) {
	if cfg.Path != "" {
		if !IsProduct(cfg.Path) {
			return nil, fmt.Errorf("claude_obsidian.path %s does not contain %s", cfg.Path, cliScript)
		}
		return &Product{Root: cfg.Path, Version: versionAt(cfg.Path), Source: "config"}, nil
	}
	inst, err := claudecode.InstalledPlugin(cfg.Plugin)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("%w: plugin %s is not installed in Claude Code", ErrNotInstalled, cfg.Plugin)
	}
	if !IsProduct(inst.InstallPath) {
		return nil, fmt.Errorf("plugin %s at %s has no %s", cfg.Plugin, inst.InstallPath, cliScript)
	}
	version := inst.Version
	if version == "" {
		version = versionAt(inst.InstallPath)
	}
	return &Product{Root: inst.InstallPath, Version: version, Source: "plugin"}, nil
}

// Tested reports whether the installed version is the one atlas was verified against.
func (p *Product) Tested() bool { return p.Version == TestedVersion }

func (p *Product) command(args ...string) *exec.Cmd {
	cmd := exec.Command("python3", append([]string{filepath.Join(p.Root, cliScript)}, args...)...)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	return cmd
}

// RunJSON runs a subcommand and decodes its JSON stdout into out, whatever the exit code.
func (p *Product) RunJSON(out any, args ...string) error {
	cmd := p.command(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if runErr != nil {
			return fmt.Errorf("claude-obsidian %s failed: %s", args[0], detail)
		}
		return fmt.Errorf("claude-obsidian %s returned unreadable output: %w", args[0], err)
	}
	return nil
}

type DoctorReport struct {
	OK     bool            `json:"ok"`
	Checks map[string]bool `json:"checks"`
}

func (p *Product) Doctor(vault string) (*DoctorReport, error) {
	var report DoctorReport
	if err := p.RunJSON(&report, "doctor", "--vault", vault); err != nil {
		return nil, err
	}
	return &report, nil
}

type LintSummary struct {
	CategoryCounts map[string]int `json:"category_counts"`
	PagesScanned   int            `json:"pages_scanned"`
	IssuesFound    int            `json:"issues_found"`
}

func (p *Product) Lint(vault string) (*LintSummary, error) {
	var report struct {
		Summary LintSummary `json:"summary"`
	}
	if err := p.RunJSON(&report, "lint", "--vault", vault, "--format", "json"); err != nil {
		return nil, err
	}
	return &report.Summary, nil
}

type InitPlan struct {
	Status       string   `json:"status"`
	ChangedPaths []string `json:"changed_paths"`
	Approval     string   `json:"approved_plan_sha256"`
}

func (p *Product) InitPlan(path, generatedAt, operation string) (*InitPlan, error) {
	var plan InitPlan
	err := p.RunJSON(&plan, "init", path, "--generated-at", generatedAt, "--operation-id", operation)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (p *Product) InitApply(path, generatedAt, operation, approval string) error {
	var result map[string]any
	return p.RunJSON(&result, "init", path,
		"--generated-at", generatedAt, "--operation-id", operation,
		"--approved-plan-sha256", approval, "--apply")
}

// NowUTC is the pinned timestamp format claude-obsidian expects.
func NowUTC() string {
	return time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}

// OperationID derives a stable operation id from a prefix and timestamp.
func OperationID(prefix, generatedAt string) string {
	r := strings.NewReplacer(":", "", "-", "")
	return prefix + "-" + r.Replace(generatedAt)
}
