package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/vault"
)

var now = time.Date(2026, 9, 12, 15, 0, 0, 0, time.UTC)

type client struct {
	t    *testing.T
	sess *mcp.ClientSession
}

func connect(t *testing.T, projectDir string) *client {
	t.Helper()
	s := New(Options{Version: "test", ProjectDir: projectDir, Env: func(string) string { return "" }, Now: func() time.Time { return now }})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return &client{t: t, sess: sess}
}

// call invokes a tool and decodes its structured result into out. It returns the error text for tool errors.
func (c *client) call(name string, args map[string]any, out any) string {
	c.t.Helper()
	res, err := c.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.t.Fatalf("%s: protocol error %v", name, err)
	}
	if res.IsError {
		var parts []string
		for _, content := range res.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				parts = append(parts, tc.Text)
			}
		}
		return strings.Join(parts, " ")
	}
	if out != nil {
		data, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s: decode %v: %s", name, err, data)
		}
	}
	return ""
}

func newVault(t *testing.T) *vault.Vault {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := filepath.Join(t.TempDir(), "v")
	if _, err := vault.Init(root, vault.Generic, now); err != nil {
		t.Fatal(err)
	}
	v, _ := vault.Open(root)
	return v
}

const page = "---\ntitle: %s\ntype: %s\nstatus: seed\ncreated: 2026-09-12\nupdated: 2026-09-12\ntags:\n  - x\n---\n# %s\n\n%s\n"

func TestToolsListAndStatus(t *testing.T) {
	v := newVault(t)
	c := connect(t, v.Path("wiki"))
	tools, err := c.sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "apply,capture,history,inbox,lint,mode,plan,route,status,undo" {
		t.Fatalf("tools %v", names)
	}
	var st Status
	if msg := c.call("status", nil, &st); msg != "" {
		t.Fatal(msg)
	}
	if st.Vault != v.Root || st.Mode != "generic" || st.Pages != 4 || !st.Git.HasHistory || st.LastOperation == nil || st.LastOperation.Kind != "setup" {
		t.Fatalf("status %+v", st)
	}
	if msg := c.call("status", map[string]any{"vault": t.TempDir()}, nil); !strings.Contains(msg, "not a claude-atlas vault") {
		t.Fatalf("non-vault: %q", msg)
	}
}

func TestIngestWorkflow(t *testing.T) {
	v := newVault(t)
	c := connect(t, v.Root)
	os.WriteFile(v.Path("inbox/paper.md"), []byte("# A paper\n\nThe claim.\n"), 0o644)

	var inbox InboxOut
	c.call("inbox", nil, &inbox)
	if len(inbox.Files) != 1 || inbox.Files[0].Captured {
		t.Fatalf("inbox %+v", inbox)
	}
	var cap struct {
		Sources []struct {
			SourceID   string `json:"source_id"`
			StoredPath string `json:"stored_path"`
		} `json:"sources"`
		Commit string `json:"commit"`
	}
	if msg := c.call("capture", map[string]any{"paths": []string{"paper.md"}}, &cap); msg != "" {
		t.Fatal(msg)
	}
	if len(cap.Sources) != 1 || cap.Commit == "" {
		t.Fatalf("capture %+v", cap)
	}
	var route vault.Route
	c.call("route", map[string]any{"type": "source", "title": "A paper"}, &route)
	if route.Path != "wiki/sources/A paper.md" || route.Exists || !strings.Contains(route.Skeleton, "type: source") {
		t.Fatalf("route %+v", route)
	}
	index, _ := os.ReadFile(v.Path(vault.IndexPage))
	newIndex := strings.Replace(string(index), "- No sources yet.", "- [[A paper]]", 1)
	var plan PlanOut
	msg := c.call("plan", map[string]any{
		"kind": "ingest", "summary": "Ingest A paper",
		"writes": []map[string]any{
			{"path": route.Path, "mode": "create", "content": strings.NewReplacer("%s", "A paper").Replace(page)},
			{"path": vault.IndexPage, "mode": "replace", "content": newIndex},
			{"path": "inbox/paper.md", "mode": "delete"},
		},
		"sources": []map[string]any{{"id": cap.Sources[0].SourceID, "ingested": true, "pages": []string{route.Path}, "authority": "primary"}},
	}, &plan)
	if msg != "" {
		t.Fatal(msg)
	}
	if len(plan.Preview.Creates) != 1 || len(plan.Preview.Replaces) != 1 || len(plan.Preview.Deletes) != 1 || len(plan.Warnings) != 0 || !strings.Contains(plan.Next, plan.PlanID) {
		t.Fatalf("plan %+v", plan)
	}
	var applied struct {
		OperationID  string   `json:"operation_id"`
		Commit       string   `json:"commit"`
		ChangedPaths []string `json:"changed_paths"`
	}
	if msg := c.call("apply", map[string]any{"plan_id": plan.PlanID}, &applied); msg != "" {
		t.Fatal(msg)
	}
	if applied.OperationID != plan.OperationID || len(applied.ChangedPaths) != 5 {
		t.Fatalf("applied %+v", applied)
	}
	if msg := c.call("apply", map[string]any{"plan_id": plan.PlanID}, nil); !strings.Contains(msg, "no plan") {
		t.Fatalf("second apply: %q", msg)
	}
	if _, err := os.Stat(v.Path("inbox/paper.md")); err == nil {
		t.Fatal("inbox file should be gone")
	}
	var hist HistoryOut
	c.call("history", map[string]any{"limit": 2}, &hist)
	if len(hist.Operations) != 2 || hist.Operations[0].ID != applied.OperationID || hist.Operations[1].Kind != "capture" {
		t.Fatalf("history %+v", hist)
	}
	var report struct {
		Summary struct {
			Pages int `json:"pages_scanned"`
		} `json:"summary"`
		DeadLinks []any `json:"dead_links"`
	}
	c.call("lint", nil, &report)
	if report.Summary.Pages != 5 || len(report.DeadLinks) != 0 {
		t.Fatalf("lint %+v", report)
	}
	var undone struct {
		Commit string `json:"commit"`
	}
	if msg := c.call("undo", map[string]any{"operation_id": applied.OperationID}, &undone); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(v.Path(route.Path)); err == nil {
		t.Fatal("undo should remove the page")
	}
}

func TestPlanErrorsAndReplacement(t *testing.T) {
	v := newVault(t)
	c := connect(t, v.Root)
	if msg := c.call("plan", map[string]any{"kind": "setup", "summary": "x", "writes": []map[string]any{{"path": "wiki/a.md", "mode": "create", "content": "x"}}}, nil); !strings.Contains(msg, "kind must be one of") {
		t.Fatalf("kind: %q", msg)
	}
	if msg := c.call("plan", map[string]any{"kind": "save", "summary": "x", "writes": []map[string]any{{"path": "wiki/a.md", "mode": "create", "content": "no front"}}}, nil); !strings.Contains(msg, "no frontmatter") {
		t.Fatalf("content: %q", msg)
	}
	content := strings.NewReplacer("%s", "B").Replace(page)
	var first, second PlanOut
	c.call("plan", map[string]any{"kind": "save", "summary": "one", "writes": []map[string]any{{"path": "wiki/B.md", "mode": "create", "content": content}}}, &first)
	c.call("plan", map[string]any{"kind": "save", "summary": "two", "writes": []map[string]any{{"path": "wiki/B.md", "mode": "create", "content": content}}}, &second)
	if msg := c.call("apply", map[string]any{"plan_id": first.PlanID}, nil); !strings.Contains(msg, "no plan") {
		t.Fatal("a newer plan for the same vault replaces the older one")
	}
	if msg := c.call("apply", map[string]any{"plan_id": second.PlanID}, nil); msg != "" {
		t.Fatal(msg)
	}
	var mode ModeOut
	c.call("mode", nil, &mode)
	if mode.Mode != "generic" || len(mode.Types) != 5 {
		t.Fatalf("mode %+v", mode)
	}
	c.call("mode", map[string]any{"set": "lyt"}, &mode)
	if mode.Plan == nil || mode.Previous != "generic" || mode.Mode != "lyt" {
		t.Fatalf("mode set %+v", mode)
	}
	if msg := c.call("apply", map[string]any{"plan_id": mode.Plan.PlanID}, nil); msg != "" {
		t.Fatal(msg)
	}
	if again, _ := vault.Open(v.Root); again.Config.Mode != vault.LYT {
		t.Fatal("mode should be lyt")
	}
	if msg := c.call("mode", map[string]any{"set": "para"}, nil); !strings.Contains(msg, "generic or lyt") {
		t.Fatalf("bad mode: %q", msg)
	}
}
