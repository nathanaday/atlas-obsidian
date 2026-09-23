package claudecode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/home"
)

// LaunchConfig is the claude_code section of config.json: Command (normally "claude"),
// Args placed before the prompt, an optional Prompt sent as the first message (for
// example "/atlas-obsidian:wiki"), and SessionContext, which lets the plugin's SessionStart
// hook hand Claude the vault's hot.md.
type LaunchConfig = home.LaunchConfig

var ErrNoClaude = errors.New("the `claude` command is not on PATH")

// EnvVault names the project's work folder for the MCP
// server and hooks; EnvSessionContext turns the session-start context on ("1") or off
// ("0") for this launch.
const (
	EnvVault          = "ATLAS_OBSIDIAN_VAULT"
	EnvSessionContext = "ATLAS_OBSIDIAN_SESSION_CONTEXT"
)

// IngestPrompt is the first message that starts an ingest of the inbox.
const IngestPrompt = "/atlas-obsidian:wiki-ingest"

// DescribePrompt is the first message that writes the page describing a project.
const DescribePrompt = "/atlas-obsidian:wiki-describe"

// ThreadPrompt is the first message that continues a thread: thread-work, which runs the
// stage after the thread's own; a closed thread opens on the board instead.
func ThreadPrompt(stage, threadID string) string {
	skill := "thread-work"
	if stage == "receipt" {
		skill = "thread"
	}
	return "/atlas-obsidian:" + skill + " " + threadID
}

// AskPrompt is the first message that opens a session on a thread and asks the user what
// to do with it.
func AskPrompt(threadID string) string {
	return "/atlas-obsidian:thread " + threadID + " Read this thread end to end, then ask me what I would like to do with it. " +
		"Offer to move it to its next stage, naming the stage and its skill, or to kill it, with or without a reason. Do nothing until I choose."
}

// PlantPrompt is the first message that plants a thread from a conversation.
const PlantPrompt = "/atlas-obsidian:thread-stub Ask me to describe the thread I want to plant, then open it from my words."

// GitPrompt is the first message that opens a session on the git state of the work.
const GitPrompt = "Help me handle the git state of this project's work folder. If it is not a git repository, offer to initialize one. " +
	"Otherwise fetch, then summarize the branch, what is uncommitted, and what is ahead of or behind the upstream. " +
	"Offer to stage changes, commit, push, pull, or anything else I want, and do nothing until I choose."

// Intent is what a session opens for. The zero Intent opens with the configured first
// message, if any.
type Intent struct {
	// Thread is a thread's id. Alone it continues the thread from Stage, its stage; with
	// Ask the session reads it and asks.
	Thread string
	Stage  string
	Ask    bool
	// Plant plants a thread; Git handles the work's git state.
	Plant bool
	Git   bool
}

// Prompt is the first message for an intent in a harness's own form: Codex names a
// skill with $ where Claude Code uses the plugin's slash command.
func Prompt(harness string, in Intent) string {
	var prompt string
	switch {
	case in.Plant:
		prompt = PlantPrompt
	case in.Git:
		prompt = GitPrompt
	case in.Thread != "" && in.Ask:
		prompt = AskPrompt(in.Thread)
	case in.Thread != "":
		prompt = ThreadPrompt(in.Stage, in.Thread)
	}
	if harness == "codex" {
		prompt = strings.Replace(prompt, "/atlas-obsidian:", "$", 1)
	}
	return prompt
}

// LaunchCommand builds the process that runs Claude Code in a project's work folder, with
// the place selected explicitly so the plugin never has to guess. prompt is the first message; empty means the configured one, if any.
func LaunchCommand(cfg LaunchConfig, place, prompt string) (*exec.Cmd, error) {
	return LaunchIn(cfg, place, place, prompt)
}

// LaunchIn is LaunchCommand with the session started in dir, while the place stays
// selected through the environment.
func LaunchIn(cfg LaunchConfig, place, dir, prompt string) (*exec.Cmd, error) {
	if dir == "" {
		dir = place
	}
	command := cfg.Command
	if command == "" {
		command = "claude"
	}
	path, err := exec.LookPath(command)
	if err != nil {
		if command != "claude" {
			return nil, fmt.Errorf("the %q command is not on PATH", command)
		}
		return nil, ErrNoClaude
	}
	args := append([]string{}, cfg.Args...)
	if prompt == "" {
		prompt = cfg.Prompt
	}
	if prompt != "" {
		args = append(args, prompt)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	context := "0"
	if cfg.SessionContext {
		context = "1"
	}
	cmd.Env = append(os.Environ(), "ATLAS_OBSIDIAN_PROJECT="+place, EnvVault+"="+place, EnvSessionContext+"="+context)
	return cmd, nil
}
