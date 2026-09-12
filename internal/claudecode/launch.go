package claudecode

import (
	"errors"
	"os"
	"os/exec"

	"github.com/nathanaday/claude-atlas/internal/home"
)

// LaunchConfig is the claude_code section of config.json: Command (normally "claude"),
// Args placed before the prompt, an optional Prompt sent as the first message (for
// example "/claude-obsidian:wiki"), and SessionContext, which lets claude-obsidian's
// SessionStart hook hand Claude the vault's hot.md.
type LaunchConfig = home.LaunchConfig

var ErrNoClaude = errors.New("the `claude` command is not on PATH")

// LaunchCommand builds the process that runs Claude Code in a vault, with the vault
// selected explicitly so claude-obsidian never has to guess.
func LaunchCommand(cfg LaunchConfig, vault string) (*exec.Cmd, error) {
	command := cfg.Command
	if command == "" {
		command = "claude"
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, ErrNoClaude
	}
	args := append([]string{}, cfg.Args...)
	if cfg.Prompt != "" {
		args = append(args, cfg.Prompt)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = vault
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "CLAUDE_OBSIDIAN_VAULT="+vault)
	if cfg.SessionContext {
		cmd.Env = append(cmd.Env,
			"CLAUDE_OBSIDIAN_SESSION_CONTEXT=1",
			"CLAUDE_OBSIDIAN_SESSION_CONTEXT_VAULT="+vault,
		)
	}
	return cmd, nil
}
