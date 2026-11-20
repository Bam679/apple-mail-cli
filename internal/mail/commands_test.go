package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-mail-cli/internal/agentcli"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestMain pins the zone so golden files match on every machine.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}

// fakeBridge is an in-memory Bridge with synthetic data only.
type fakeBridge struct {
	boxes []Mailbox
	msgs  []Message
	next  int
	fail  error
}

func newFake() *fakeBridge {
	d := func(day int) time.Time { return time.Date(2026, 11, day, 8, 0, 0, 0, time.UTC) }
	src := "Content-Type: text/plain\r\n\r\nHello Alice,\r\nthe invoice is attached.\r\n"
	return &fakeBridge{
		boxes: []Mailbox{{Path: "Work/INBOX", Account: "Work", Unread: 2}, {Path: "Work/Archive", Account: "Work"},
			{Path: "Work/Private", Account: "Work"}, {Path: "On My Mac/Agent", Account: "On My Mac"}},
		msgs: []Message{
			{ID: "101", Subject: "Invoice 42", From: "Billing <billing@example.com>", Date: d(18), Mailbox: "Work/INBOX",
				MessageID: "<a1@example.com>", To: []string{"alice@example.org"}, Size: 2048, Source: src,
				Attachments: []Attachment{{Name: "invoice.pdf", Size: 1000, MIME: "application/pdf"}}},
			{ID: "102", Subject: "Lunch?", From: "Bob <bob@example.net>", Date: d(19), Mailbox: "Work/INBOX", Flagged: true, MessageID: "<a2@example.com>"},
			{ID: "103", Subject: "Old news", From: "news@example.com", Date: d(2), Mailbox: "Work/INBOX", Read: true, MessageID: "<a3@example.com>"},
			{ID: "104", Subject: "Medical results", From: "clinic@example.com", Date: d(17), Mailbox: "Work/Private", MessageID: "<a4@example.com>"},
			{ID: "105", Subject: "Huge", From: "x@example.com", Date: d(1), Mailbox: "Work/Archive", MessageID: "<a5@example.com>", SourceSkipped: "too large"},
		},
	}
}

func (f *fakeBridge) Accounts(context.Context) ([]Account, error) {
	return []Account{{Name: "Work", Emails: []string{"alice@example.org"}, Enabled: true}}, f.fail
}
func (f *fakeBridge) Mailboxes(context.Context) ([]Mailbox, error) { return f.boxes, f.fail }
func (f *fakeBridge) Inboxes(context.Context) ([]string, error) {
	return []string{"Work/INBOX"}, f.fail
}

func (f *fakeBridge) Messages(_ context.Context, q Query) (Page, error) {
	if f.fail != nil {
		return Page{}, f.fail
	}
	var hits []Message
	for _, m := range f.msgs {
		if !has(q.Paths, m.Mailbox) || (q.Unread && m.Read) || (q.Flagged && !m.Flagged) ||
			(!q.Since.IsZero() && m.Date.Before(q.Since)) || (!q.Before.IsZero() && !m.Date.Before(q.Before)) {
			continue
		}
		if q.Text != "" {
			t := strings.ToLower(q.Text)
			if !strings.Contains(strings.ToLower(m.Subject), t) && (q.SubjectOnly || !strings.Contains(strings.ToLower(m.From), t)) {
				continue
			}
		}
		s := m
		s.MessageID, s.To, s.Source, s.Attachments = "", nil, "", nil
		hits = append(hits, s)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Date.After(hits[j].Date) })
	end := q.Offset + q.Limit
	if end > len(hits) {
		end = len(hits)
	}
	if q.Offset > len(hits) {
		q.Offset = len(hits)
	}
	return Page{Total: len(hits), Items: hits[q.Offset:end]}, nil
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (f *fakeBridge) idx(id string) int {
	for i, m := range f.msgs {
		if m.ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeBridge) Get(_ context.Context, id string, withSource bool) (Message, error) {
	i := f.idx(id)
	if i < 0 {
		return Message{}, agentcli.NotFoundf("no message with id %s", id)
	}
	m := f.msgs[i]
	if !withSource {
		m.Source = ""
	}
	return m, nil
}

func (f *fakeBridge) Mark(_ context.Context, id string, read, flagged *bool) (Message, error) {
	if f.fail != nil {
		return Message{}, f.fail
	}
	i := f.idx(id)
	if read != nil {
		f.msgs[i].Read = *read
	}
	if flagged != nil {
		f.msgs[i].Flagged = *flagged
	}
	return f.msgs[i], nil
}

func (f *fakeBridge) Move(_ context.Context, id, to string) (Message, error) {
	i := f.idx(id)
	f.next++
	f.msgs[i].Mailbox, f.msgs[i].ID = to, "9"+id
	return f.msgs[i], nil
}

func (f *fakeBridge) Delete(_ context.Context, id string) (Message, error) {
	i := f.idx(id)
	m := f.msgs[i]
	f.msgs = append(f.msgs[:i], f.msgs[i+1:]...)
	return m, nil
}

func (f *fakeBridge) CreateMailbox(_ context.Context, name string) (Mailbox, error) {
	m := Mailbox{Path: "On My Mac/" + name, Account: "On My Mac"}
	f.boxes = append(f.boxes, m)
	return m, nil
}

var (
	readOnly = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	triageRW = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, "mark", "move", "mailbox-create"}, Deny: []string{"delete"}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}, Deny: []string{"*/Private"}},
			Write: agentcli.Rule{Allow: []string{"Work/INBOX", "Work/Archive", "On My Mac/*"}}}}}}
)

func run(t *testing.T, b *fakeBridge, pol agentcli.Effective, args ...string) (int, string, string) {
	t.Helper()
	app := NewApp(b, "test")
	app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
	app.Now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }
	var out, errb bytes.Buffer
	code := app.Main(context.Background(), args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(p), 0o750)
		if err := os.WriteFile(p, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p) // #nosec G304 -- test fixture
	if err != nil {
		t.Fatalf("missing golden %s: run go test -update", p)
	}
	if got != string(want) {
		t.Fatalf("golden %s mismatch:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestReadCommandsGolden(t *testing.T) {
	cases := map[string][]string{
		"accounts.json":   {"accounts", "--json"},
		"mailboxes.txt":   {"mailboxes"},
		"list.txt":        {"list"},
		"list_unread.txt": {"list", "--unread", "--since", "2026-11-10"},
		"list_page.json":  {"list", "--limit", "1", "--json"},
		"search.ndjson":   {"search", "invoice", "--mailbox", "Work/INBOX", "--ndjson"},
		"get.txt":         {"get", "101"},
		"get_nobody.json": {"get", "101", "--no-body", "--json"},
		"get_big.json":    {"get", "105", "--json", "--fields", "id,body,body_note"},
		"doctor.txt":      {"doctor"},
	}
	for name, args := range cases {
		code, out, errOut := run(t, newFake(), triageRW, args...)
		if code != 0 {
			t.Fatalf("%s: exit %d %s", name, code, errOut)
		}
		golden(t, name, out)
	}
}

func TestPagination(t *testing.T) {
	_, out, _ := run(t, newFake(), triageRW, "list", "--limit", "2", "--json")
	var p struct {
		Items      []map[string]any
		NextCursor string `json:"next_cursor"`
	}
	_ = json.Unmarshal([]byte(out), &p)
	if len(p.Items) != 2 || p.NextCursor == "" {
		t.Fatalf("page1: %s", out)
	}
	_, out, _ = run(t, newFake(), triageRW, "list", "--limit", "2", "--json", "--cursor", p.NextCursor)
	var p2 struct {
		Items      []map[string]any
		NextCursor string `json:"next_cursor"`
	}
	_ = json.Unmarshal([]byte(out), &p2)
	if len(p2.Items) != 1 || p2.NextCursor != "" || p2.Items[0]["id"] != "103" {
		t.Fatalf("page2: %s", out)
	}
	if code, _, _ := run(t, newFake(), triageRW, "list", "--unread", "--limit", "2", "--cursor", p.NextCursor); code != agentcli.ExitUsage {
		t.Fatal("cursor reuse with other filters must fail")
	}
}

func TestReadScope(t *testing.T) {
	_, out, _ := run(t, newFake(), triageRW, "mailboxes")
	if strings.Contains(out, "Private") {
		t.Fatal("private mailbox listed")
	}
	if code, _, _ := run(t, newFake(), triageRW, "get", "104"); code != agentcli.ExitPolicy {
		t.Fatal("get private")
	}
	if code, _, _ := run(t, newFake(), triageRW, "list", "--mailbox", "Work/Private"); code != agentcli.ExitPolicy {
		t.Fatal("list private")
	}
	pol := agentcli.Effective{Layers: []agentcli.Policy{{Version: 1, Operations: agentcli.OpRule{Allow: []string{"read"}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"On My Mac/*"}}}}}}
	if code, out, _ := run(t, newFake(), pol, "list"); code != 0 || out != "" {
		t.Fatalf("no readable inbox must give empty list: %d %q", code, out)
	}
}

func TestPolicyDeniedWriteIsBlocked(t *testing.T) {
	b := newFake()
	for _, args := range [][]string{
		{"mark", "101", "--read", "--confirm"},
		{"mark", "101", "--read", "--dry-run"},
		{"move", "101", "--to", "Work/Archive", "--confirm"},
		{"mailbox-create", "Agent2", "--confirm"},
	} {
		if code, _, e := run(t, b, readOnly, args...); code != agentcli.ExitPolicy {
			t.Fatalf("%v: %d %s", args, code, e)
		}
	}
	if code, _, _ := run(t, b, triageRW, "delete", "101", "--confirm"); code != agentcli.ExitPolicy {
		t.Fatal("op deny must win")
	}
	if code, _, _ := run(t, b, triageRW, "move", "101", "--to", "Work/Private", "--confirm"); code != agentcli.ExitPolicy {
		t.Fatal("target outside write scope")
	}
	if b.msgs[0].Read || b.msgs[0].Mailbox != "Work/INBOX" || len(b.msgs) != 5 {
		t.Fatal("denied write changed data")
	}
}

func TestWriteFlow(t *testing.T) {
	b := newFake()
	if code, out, _ := run(t, b, triageRW, "mark", "101", "102", "--read", "--unflag", "--dry-run"); code != 0 || b.msgs[0].Read || !strings.Contains(out, "true") {
		t.Fatalf("dry-run: %s", out)
	}
	if code, _, _ := run(t, b, triageRW, "mark", "101", "--read"); code != agentcli.ExitConfirm {
		t.Fatal("confirm")
	}
	if code, _, _ := run(t, b, triageRW, "mark", "101", "102", "--read", "--flag", "--confirm"); code != 0 || !b.msgs[0].Read || !b.msgs[0].Flagged {
		t.Fatal("mark")
	}
	if code, _, _ := run(t, b, triageRW, "mark", "101", "--unread", "--unflag", "--confirm"); code != 0 || b.msgs[0].Read || b.msgs[0].Flagged {
		t.Fatal("unmark")
	}
	if code, out, _ := run(t, b, triageRW, "move", "101", "--to", "Work/Archive", "--confirm", "--json"); code != 0 || !strings.Contains(out, `"id":"9101"`) {
		t.Fatalf("move returns new id: %s", out)
	}
	if code, _, _ := run(t, b, triageRW, "mailbox-create", "Agent2", "--confirm"); code != 0 {
		t.Fatal("mailbox-create")
	}
	if code, _, _ := run(t, b, triageRW, "mailbox-create", "Agent2", "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("duplicate mailbox")
	}
	all := agentcli.Effective{Layers: []agentcli.Policy{{Version: 1, Operations: agentcli.OpRule{Allow: []string{"read", "write"}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}}, Write: agentcli.Rule{Allow: []string{"*"}}}}}}
	if code, _, _ := run(t, b, all, "delete", "102", "--confirm"); code != 0 || b.idx("102") >= 0 {
		t.Fatal("delete")
	}
}

func TestValidation(t *testing.T) {
	for name, args := range map[string][]string{
		"no mark flag":  {"mark", "101", "--dry-run"},
		"contradiction": {"mark", "101", "--read", "--unread", "--dry-run"},
		"flag contra":   {"mark", "101", "--flag", "--unflag", "--dry-run"},
		"slash name":    {"mailbox-create", "a/b", "--dry-run"},
		"empty search":  {"search", " "},
		"bad since":     {"list", "--since", "x"},
		"no send":       {"send", "--to", "x"},
	} {
		if code, _, _ := run(t, newFake(), triageRW, args...); code != agentcli.ExitUsage {
			t.Errorf("%s: %d", name, code)
		}
	}
	if code, _, _ := run(t, newFake(), triageRW, "move", "101", "--to", "Nope/X", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown target")
	}
	if code, _, _ := run(t, newFake(), triageRW, "list", "--mailbox", "Nope/X"); code != agentcli.ExitNotFound {
		t.Error("unknown mailbox")
	}
	if code, _, _ := run(t, newFake(), triageRW, "mark", "999", "--read", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown id")
	}
}

func TestBridgeErrors(t *testing.T) {
	b := newFake()
	b.fail = agentcli.Permissionf("denied")
	for _, args := range [][]string{{"accounts"}, {"mailboxes"}, {"list"}, {"search", "x", "--mailbox", "Work/INBOX"}, {"move", "101", "--to", "x", "--dry-run"}, {"mailbox-create", "x", "--dry-run"}} {
		if code, _, _ := run(t, b, triageRW, args...); code != agentcli.ExitPermission {
			t.Errorf("%v: %d", args, code)
		}
	}
	if code, out, _ := run(t, b, triageRW, "doctor"); code != agentcli.ExitPermission || !strings.Contains(out, "false") {
		t.Error("doctor")
	}
	b2 := newFake()
	b2.fail = agentcli.Unavailablef("x")
	if code, _, _ := run(t, b2, triageRW, "get", "101"); code != 0 {
		t.Fatal("get ignores fail")
	}
	e := agentcli.AsError(partial(agentcli.Unavailablef("boom"), 1, 2))
	if !strings.HasPrefix(e.Message, "1 of 2") {
		t.Fatal("partial")
	}
}

func TestNoSendCommandExists(t *testing.T) {
	_, out, _ := run(t, newFake(), readOnly, "schema")
	for _, forbidden := range []string{`"send"`, `"reply"`, `"forward"`, `"draft"`} {
		if strings.Contains(out, `"name": `+forbidden) {
			t.Fatalf("schema must not contain %s", forbidden)
		}
	}
	if strings.Contains(script, "send(") || strings.Contains(script, "outgoingMessage") || strings.Contains(script, "OutgoingMessage") {
		t.Fatal("bridge script must not be able to send mail")
	}
}
