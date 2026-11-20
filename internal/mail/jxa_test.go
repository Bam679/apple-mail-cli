package mail

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Bam679/apple-mail-cli/internal/osa"
)

func TestScriptIsStatic(t *testing.T) {
	if !strings.Contains(script, "const OPS") || strings.Contains(script, "%s") {
		t.Fatal("static OPS table expected")
	}
	for _, op := range []string{"accounts", "mailboxes", "inboxes", "messages", "get", "mark", "move", "delete", "mailbox_create", "draft"} {
		if !strings.Contains(script, "  "+op+": function") {
			t.Errorf("missing op %s", op)
		}
	}
}

func TestJXAWireMapping(t *testing.T) {
	ms := int64(1767225600000)
	wire := map[string]any{"id": "7", "subject": "S", "from": "F", "date_ms": ms, "read": true, "mailbox": "A/INBOX",
		"message_id": "<m>", "to": []string{"t"}, "date_sent_ms": nil, "size": 9, "attachments": []Attachment{{Name: "a"}}, "source": "x"}
	seen := map[string]string{}
	f := &osa.Fake{Handler: func(op string, args json.RawMessage) (any, error) {
		seen[op] = string(args)
		switch op {
		case "accounts":
			return []Account{{Name: "A"}}, nil
		case "mailboxes":
			return []Mailbox{{Path: "A/INBOX"}}, nil
		case "inboxes":
			return []string{"A/INBOX"}, nil
		case "messages":
			return map[string]any{"total": 3, "items": []any{wire}}, nil
		case "mailbox_create":
			return Mailbox{Path: "On My Mac/x"}, nil
		}
		return wire, nil
	}}
	j := &JXA{Runner: f}
	ctx := context.Background()
	if _, err := j.Accounts(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Mailboxes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Inboxes(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := j.Messages(ctx, Query{Paths: []string{"A/INBOX"}, Limit: 5, Text: "q"})
	if err != nil || p.Total != 3 || len(p.Items) != 1 || p.Items[0].Date.UnixMilli() != ms || !p.Items[0].DateSent.IsZero() {
		t.Fatalf("messages: %+v %v", p, err)
	}
	if !strings.Contains(seen["messages"], `"since_ms":0`) || !strings.Contains(seen["messages"], `"query":"q"`) {
		t.Fatal(seen["messages"])
	}
	m, err := j.Get(ctx, "7", true)
	if err != nil || m.Source != "x" || len(m.Attachments) != 1 || !strings.Contains(seen["get"], `"max_source":10485760`) {
		t.Fatalf("get: %+v %v", m, err)
	}
	r := true
	if _, err := j.Mark(ctx, "7", &r, nil); err != nil || !strings.Contains(seen["mark"], `"flagged":null`) {
		t.Fatal(seen["mark"])
	}
	if _, err := j.Move(ctx, "7", "A/Archive"); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Delete(ctx, "7"); err != nil {
		t.Fatal(err)
	}
	if _, err := j.SaveDraft(ctx, Draft{Subject: "s", To: []string{"a@example.com"}}); err != nil || !strings.Contains(seen["draft"], `"to":["a@example.com"]`) {
		t.Fatalf("draft args: %s %v", seen["draft"], err)
	}
	if _, err := j.CreateMailbox(ctx, "x"); err != nil || NewJXA().Runner == nil {
		t.Fatal(err)
	}
	fe := &osa.Fake{Handler: func(string, json.RawMessage) (any, error) { return nil, context.Canceled }}
	if _, err := (&JXA{Runner: fe}).Messages(ctx, Query{}); err == nil {
		t.Fatal("error must propagate")
	}
	if _, err := (&JXA{Runner: fe}).Get(ctx, "1", false); err == nil {
		t.Fatal("error must propagate")
	}
}
