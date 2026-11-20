//go:build integration && darwin

package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-mail-cli/internal/agentcli"
)

// sandbox is a persistent local mailbox. Mail's scripting interface cannot
// delete mailboxes (error -10000), so it is created once and reused; it
// lives under "On My Mac" and never syncs to a server.
const sandbox = "On My Mac/agent-cli-test-sandbox"

// The test never changes real messages and never sends mail. Real messages are
// only read (headers, body length) and used as targets of writes that the
// policy must block; nothing is printed from them.
func TestIntegrationMail(t *testing.T) {
	ctx := context.Background()
	b := NewJXA()
	pol := agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	cli := func(args ...string) (int, string) {
		app := NewApp(b, "it")
		app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
		var out, errb bytes.Buffer
		code := app.Main(ctx, args, strings.NewReader(""), &out, &errb)
		if code != 0 {
			t.Logf("%v -> %d %s", args[0], code, errb.String())
		}
		return code, out.String()
	}

	code, out := cli("mailboxes", "--json")
	if code != 0 {
		t.Fatal("mailboxes")
	}
	if !strings.Contains(out, sandbox) {
		pol = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
			Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, "mailbox-create"}},
			Scopes:     agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}}, Write: agentcli.Rule{Allow: []string{sandbox}}}}}}
		if code, _ = cli("mailbox-create", "agent-cli-test-sandbox", "--confirm"); code != 0 {
			t.Fatal("creating the sandbox mailbox failed")
		}
		pol = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	}
	if code, _ = cli("list", "--mailbox", sandbox); code != 0 {
		t.Fatal("list sandbox")
	}
	if code, _ = cli("search", "agent-cli", "--mailbox", sandbox, "--subject-only"); code != 0 {
		t.Fatal("search sandbox")
	}

	// Writes against the sandbox scope: the duplicate create is a conflict.
	pol = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
		Scopes:     agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}}, Write: agentcli.Rule{Allow: []string{sandbox}}}}}}
	if code, _ = cli("mailbox-create", "agent-cli-test-sandbox", "--confirm"); code != agentcli.ExitConflict {
		t.Fatalf("duplicate sandbox create: %d", code)
	}

	// Read the newest real inbox message (if any) without printing it.
	code, out = cli("list", "--limit", "1", "--json")
	if code != 0 {
		t.Fatal("list inboxes")
	}
	var page struct{ Items []map[string]any }
	if err := json.Unmarshal([]byte(out), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) == 0 {
		t.Log("no inbox messages: skipping real-message checks")
		return
	}
	id := page.Items[0]["id"].(string)
	before, err := b.Get(ctx, id, false)
	if err != nil {
		t.Fatal(err)
	}
	if code, out = cli("get", id, "--json", "--max-chars", "2000"); code != 0 || !strings.Contains(out, `"message_id"`) {
		t.Fatal("get with body")
	}

	// Moving a real message into the sandbox is denied: its source mailbox is
	// outside the write scope. Marking is denied by the read-only default.
	if code, _ = cli("move", id, "--to", sandbox, "--confirm"); code != agentcli.ExitPolicy {
		t.Fatalf("move from real mailbox must be denied: %d", code)
	}
	pol = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	flip := "--read"
	if before.Read {
		flip = "--unread"
	}
	if code, _ = cli("mark", id, flip, "--confirm"); code != agentcli.ExitPolicy {
		t.Fatalf("mark must be denied: %d", code)
	}
	after, err := b.Get(ctx, id, false)
	if err != nil || after.Read != before.Read || after.Mailbox != before.Mailbox {
		t.Fatal("a denied write changed the real message")
	}
}

// TestIntegrationDraft saves one draft with a unique subject in the default
// account, finds it and moves it to the trash. It writes to a server-synced
// Drafts mailbox, so it only runs with AGENT_CLI_MAIL_DRAFT_TEST=1.
func TestIntegrationDraft(t *testing.T) {
	if os.Getenv("AGENT_CLI_MAIL_DRAFT_TEST") != "1" {
		t.Skip("set AGENT_CLI_MAIL_DRAFT_TEST=1 to save and delete a real draft")
	}
	ctx := context.Background()
	b := NewJXA()
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	token := "agent-cli-test-draft-" + hex.EncodeToString(buf)
	app := NewApp(b, "it")
	app.LoadPolicy = func() (agentcli.Effective, error) {
		return agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}, nil
	}
	var out, errb bytes.Buffer
	code := app.Main(ctx, []string{"draft", "--subject", token, "--body", "Synthetic test draft, safe to delete.", "--confirm", "--json"}, strings.NewReader(""), &out, &errb)
	if code != 0 || !strings.Contains(out.String(), `"saved":true`) {
		t.Fatalf("draft with the default policy: %d %s", code, errb.String())
	}
	boxes, err := b.Mailboxes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, m := range boxes {
		paths = append(paths, m.Path)
	}
	var found Message
	for i := 0; i < 20 && found.ID == ""; i++ {
		page, err := b.Messages(ctx, Query{Paths: paths, Limit: 5, Text: token, SubjectOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) > 0 {
			found = page.Items[0]
			break
		}
		time.Sleep(time.Second)
	}
	if found.ID == "" {
		t.Fatal("saved draft not found in any mailbox")
	}
	t.Logf("draft saved in %s", found.Mailbox)
	if _, err := b.Delete(ctx, found.ID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
