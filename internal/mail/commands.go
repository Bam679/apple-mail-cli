package mail

import (
	"context"
	netmail "net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Bam679/apple-mail-cli/internal/agentcli"
)

const localAccount = "On My Mac"

var (
	msgFields = []string{"id", "date", "from", "subject", "mailbox", "read", "flagged", "message_id", "to", "cc",
		"date_sent", "size", "junk", "attachments", "body", "body_note"}
	msgCols = []string{"id", "date", "from", "subject", "mailbox"}
)

// NewApp wires the CLI commands to bridge b.
func NewApp(b Bridge, version string) *agentcli.App {
	h := &handlers{b: b}
	boxFlag := agentcli.Flag{Name: "mailbox", Type: agentcli.Strings, Usage: "mailbox path Account/Box (repeatable; default: all inboxes)"}
	filters := []agentcli.Flag{boxFlag,
		{Name: "unread", Type: agentcli.Bool, Usage: "only unread messages"},
		{Name: "flagged", Type: agentcli.Bool, Usage: "only flagged messages"},
		{Name: "since", Type: agentcli.String, Usage: "received on/after this date"},
		{Name: "before", Type: agentcli.String, Usage: "received before this date"}}
	return &agentcli.App{
		Name:         "apple-mail",
		Version:      version,
		Summary:      "Apple Mail for AI agents: read, triage and draft mail; never sends",
		ScopeHelp:    "mailbox path \"Account/Mailbox/Sub\" (local: \"On My Mac/Name\")",
		ScopeExample: "On My Mac/Agent*",
		DefaultOn:    []string{"draft"},
		Commands: []*agentcli.Command{
			{Name: "accounts", Summary: "list Mail accounts", Kind: agentcli.Read,
				Fields: []string{"name", "enabled"}, AllFields: []string{"name", "enabled", "emails"}, Run: h.accounts},
			{Name: "mailboxes", Summary: "list mailboxes with unread counts", Kind: agentcli.Read,
				Flags:  []agentcli.Flag{{Name: "account", Type: agentcli.String, Usage: "only this account"}},
				Fields: []string{"path", "unread"}, AllFields: []string{"path", "account", "unread"}, Run: h.mailboxes},
			{Name: "list", Summary: "list messages newest first (headers only)", Kind: agentcli.Read,
				Flags: filters, Fields: msgCols, AllFields: msgFields, Run: h.list,
				Example: "apple-mail list --unread --since -3d --limit 20"},
			{Name: "search", Summary: "find messages whose subject or sender contains text", Kind: agentcli.Read,
				Args:   []agentcli.Arg{{Name: "query", Usage: "case-insensitive text", Required: true}},
				Flags:  append(append([]agentcli.Flag{}, filters...), agentcli.Flag{Name: "subject-only", Type: agentcli.Bool, Usage: "match the subject only"}),
				Fields: msgCols, AllFields: msgFields, Run: h.search, Example: "apple-mail search invoice --since -30d --json"},
			{Name: "get", Summary: "show one message with plain-text body", Kind: agentcli.Read,
				Args:   []agentcli.Arg{{Name: "id", Usage: "message id", Required: true}},
				Flags:  []agentcli.Flag{{Name: "no-body", Type: agentcli.Bool, Usage: "headers and attachment list only (fast)"}},
				Fields: []string{"id", "date", "from", "to", "subject", "mailbox", "attachments", "body"}, AllFields: msgFields, Run: h.get,
				Example: "apple-mail get 123456 --max-chars 6000"},
			{Name: "doctor", Summary: "check permissions and connectivity", Kind: agentcli.Read,
				Fields: []string{"check", "ok", "detail"}, Run: h.doctor},
			{Name: "draft", Summary: "save a new draft in Mail's Drafts mailbox (never sends)", Kind: agentcli.Write, Op: "draft",
				Args: []agentcli.Arg{{Name: "input", Usage: "'-' to read JSON objects from stdin"}},
				Flags: []agentcli.Flag{
					{Name: "to", Type: agentcli.Strings, Usage: "recipient address (repeatable)"},
					{Name: "cc", Type: agentcli.Strings, Usage: "cc address (repeatable)"},
					{Name: "bcc", Type: agentcli.Strings, Usage: "bcc address (repeatable)"},
					{Name: "subject", Type: agentcli.String, Usage: "subject"},
					{Name: "body", Type: agentcli.String, Usage: "plain-text body"},
					{Name: "from", Type: agentcli.String, Usage: "sender address of one of your accounts (default: Mail's default)"}},
				Stdin:  `{"to":["a@example.com"],"cc":[],"bcc":[],"subject":"…","body":"…","from":"me@example.com"} (one per line)`,
				Fields: []string{"from", "to", "subject"}, AllFields: []string{"from", "to", "cc", "subject", "saved"}, Plan: h.draft,
				Example: "apple-mail draft --to bob@example.com --subject 'Re: invoice' --body 'Thanks, paid today.' --confirm"},
			{Name: "mark", Summary: "set read/unread and flagged/unflagged", Kind: agentcli.Write, Op: "mark",
				Args: []agentcli.Arg{{Name: "id", Usage: "message id(s)", Required: true, Variadic: true}},
				Flags: []agentcli.Flag{{Name: "read", Type: agentcli.Bool, Usage: "mark read"}, {Name: "unread", Type: agentcli.Bool, Usage: "mark unread"},
					{Name: "flag", Type: agentcli.Bool, Usage: "flag"}, {Name: "unflag", Type: agentcli.Bool, Usage: "remove flag"}},
				Fields: msgCols, AllFields: msgFields, Plan: h.mark},
			{Name: "move", Summary: "move messages to another mailbox (ids change)", Kind: agentcli.Write, Op: "move",
				Args:   []agentcli.Arg{{Name: "id", Usage: "message id(s)", Required: true, Variadic: true}},
				Flags:  []agentcli.Flag{{Name: "to", Type: agentcli.String, Usage: "target mailbox path", Required: true}},
				Fields: msgCols, AllFields: msgFields, Plan: h.move},
			{Name: "delete", Summary: "move messages to the account's trash", Kind: agentcli.Write, Op: "delete",
				Args:   []agentcli.Arg{{Name: "id", Usage: "message id(s)", Required: true, Variadic: true}},
				Fields: msgCols, AllFields: msgFields, Plan: h.delete},
			{Name: "mailbox-create", Summary: "create a local mailbox under \"On My Mac\"", Kind: agentcli.Write, Op: "mailbox-create",
				Args:   []agentcli.Arg{{Name: "name", Usage: "mailbox name", Required: true}},
				Fields: []string{"path"}, AllFields: []string{"path", "account", "unread"}, Plan: h.mailboxCreate},
		},
	}
}

type handlers struct{ b Bridge }

func msgItem(m Message) agentcli.Item {
	it := agentcli.Item{"id": m.ID, "date": agentcli.FormatTime(m.Date, false), "from": m.From, "subject": m.Subject,
		"mailbox": m.Mailbox, "read": m.Read, "flagged": m.Flagged}
	if m.MessageID != "" {
		var atts []string
		for _, a := range m.Attachments {
			atts = append(atts, a.Name+" ("+a.MIME+", "+strconv.FormatInt(a.Size, 10)+" B)")
		}
		it["message_id"], it["to"], it["cc"] = m.MessageID, nonNil(m.To), nonNil(m.CC)
		it["date_sent"], it["size"], it["junk"], it["attachments"] = agentcli.FormatTime(m.DateSent, false), m.Size, m.Junk, nonNil(atts)
	}
	return it
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (h *handlers) accounts(c *agentcli.Ctx) error {
	as, err := h.b.Accounts(c)
	if err != nil {
		return err
	}
	var items []agentcli.Item
	for _, a := range as {
		items = append(items, agentcli.Item{"name": a.Name, "enabled": a.Enabled, "emails": nonNil(a.Emails)})
	}
	return c.List(items)
}

func (h *handlers) mailboxes(c *agentcli.Ctx) error {
	ms, err := h.b.Mailboxes(c)
	if err != nil {
		return err
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Path < ms[j].Path })
	var items []agentcli.Item
	for _, m := range ms {
		if c.CanRead(m.Path) && (!c.IsSet("account") || m.Account == c.String("account")) {
			items = append(items, agentcli.Item{"path": m.Path, "account": m.Account, "unread": m.Unread})
		}
	}
	return c.List(items, c.String("account"))
}

// paths resolves --mailbox (validated against existing mailboxes) or the
// readable inboxes.
func (h *handlers) paths(c *agentcli.Ctx) ([]string, error) {
	refs := c.Strings("mailbox")
	if len(refs) == 0 {
		inboxes, err := h.b.Inboxes(c)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, p := range inboxes {
			if c.CanRead(p) {
				out = append(out, p)
			}
		}
		return out, nil
	}
	ms, err := h.b.Mailboxes(c)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, m := range ms {
		known[m.Path] = true
	}
	for _, r := range refs {
		if !known[r] {
			return nil, agentcli.NotFoundf("no mailbox %q (see: apple-mail mailboxes)", r)
		}
		if err := c.CheckRead(r); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func (h *handlers) query(c *agentcli.Ctx, text string) error {
	q := Query{Unread: c.Bool("unread"), Flagged: c.Bool("flagged"), Text: text, SubjectOnly: c.Bool("subject-only")}
	for flag, dst := range map[string]*time.Time{"since": &q.Since, "before": &q.Before} {
		if s := c.String(flag); s != "" {
			t, _, err := agentcli.ParseTime(s, c.Now(), time.Local)
			if err != nil {
				return err
			}
			*dst = t
		}
	}
	var err error
	if q.Paths, err = h.paths(c); err != nil {
		return err
	}
	key := agentcli.QueryKey(c.Cmd.Name, strconv.Itoa(c.Limit), strings.Join(q.Paths, "\x00"), text, c.String("since"), c.String("before"),
		strconv.FormatBool(q.Unread), strconv.FormatBool(q.Flagged), strconv.FormatBool(q.SubjectOnly))
	if q.Offset, err = agentcli.DecodeCursor(c.Cursor, key); err != nil {
		return err
	}
	q.Limit = c.Limit
	page := Page{}
	if len(q.Paths) > 0 {
		if page, err = h.b.Messages(c, q); err != nil {
			return err
		}
	}
	var items []agentcli.Item
	for _, m := range page.Items {
		if c.CanRead(m.Mailbox) {
			items = append(items, msgItem(m))
		}
	}
	p := agentcli.Page{Items: items}
	if next := q.Offset + len(page.Items); next < page.Total {
		p.NextCursor = agentcli.EncodeCursor(next, key)
	}
	return c.Out.List(p, c.Cmd.Fields)
}

func (h *handlers) list(c *agentcli.Ctx) error { return h.query(c, "") }

func (h *handlers) search(c *agentcli.Ctx) error {
	if strings.TrimSpace(c.Args[0]) == "" {
		return agentcli.Usagef("query must not be empty")
	}
	return h.query(c, c.Args[0])
}

func (h *handlers) get(c *agentcli.Ctx) error {
	m, err := h.b.Get(c, c.Args[0], !c.Bool("no-body"))
	if err != nil {
		return err
	}
	if err := c.CheckRead(m.Mailbox); err != nil {
		return err
	}
	it := msgItem(m)
	switch {
	case m.SourceSkipped != "":
		it["body_note"] = m.SourceSkipped
	case m.Source != "":
		it["body"] = BodyText([]byte(m.Source))
	}
	return c.Show(it)
}

func (h *handlers) doctor(c *agentcli.Ctx) error {
	ms, err := h.b.Mailboxes(c)
	row := agentcli.Item{"check": "mail_automation", "ok": err == nil}
	if err != nil {
		row["detail"] = agentcli.AsError(err).Message
	} else {
		row["detail"] = strconv.Itoa(len(ms)) + " mailboxes visible"
	}
	if lerr := c.List([]agentcli.Item{row}); lerr != nil {
		return lerr
	}
	return err
}

// partial annotates an error that happened after some writes succeeded.
func partial(err error, done, total int) error {
	if done == 0 {
		return err
	}
	e := *agentcli.AsError(err)
	e.Message = strconv.Itoa(done) + " of " + strconv.Itoa(total) + " writes succeeded before: " + e.Message
	return &e
}

// each loads every id (headers only) and builds a plan applying fn.
func (h *handlers) each(c *agentcli.Ctx, extra []string, fn func(context.Context, Message) (Message, error)) (*agentcli.WritePlan, error) {
	var ms []Message
	scopes := append([]string{}, extra...)
	var targets []agentcli.Item
	for _, id := range c.Args {
		m, err := h.b.Get(c, id, false)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
		scopes = append(scopes, m.Mailbox)
		targets = append(targets, agentcli.Item{"id": m.ID, "date": agentcli.FormatTime(m.Date, false), "from": m.From, "subject": m.Subject, "mailbox": m.Mailbox})
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, m := range ms {
			u, err := fn(ctx, m)
			if err != nil {
				return nil, partial(err, len(out), len(ms))
			}
			u.MessageID = ""
			out = append(out, msgItem(u))
		}
		return out, nil
	}}, nil
}

func (h *handlers) mark(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	var read, flagged *bool
	t, f := true, false
	switch {
	case c.Bool("read") && c.Bool("unread"), c.Bool("flag") && c.Bool("unflag"):
		return nil, agentcli.Usagef("contradicting flags")
	case c.Bool("read"):
		read = &t
	case c.Bool("unread"):
		read = &f
	}
	if c.Bool("flag") {
		flagged = &t
	} else if c.Bool("unflag") {
		flagged = &f
	}
	if read == nil && flagged == nil {
		return nil, agentcli.Usagef("pass --read, --unread, --flag or --unflag")
	}
	p, err := h.each(c, nil, func(ctx context.Context, m Message) (Message, error) { return h.b.Mark(ctx, m.ID, read, flagged) })
	if err != nil {
		return nil, err
	}
	for _, t := range p.Targets {
		if read != nil {
			t["read"] = *read
		}
		if flagged != nil {
			t["flagged"] = *flagged
		}
	}
	return p, nil
}

func (h *handlers) move(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	to := c.String("to")
	ms, err := h.b.Mailboxes(c)
	if err != nil {
		return nil, err
	}
	found := false
	for _, m := range ms {
		found = found || m.Path == to
	}
	if !found {
		return nil, agentcli.NotFoundf("no mailbox %q", to)
	}
	p, err := h.each(c, []string{to}, func(ctx context.Context, m Message) (Message, error) { return h.b.Move(ctx, m.ID, to) })
	if err != nil {
		return nil, err
	}
	for _, t := range p.Targets {
		t["to"] = to
	}
	return p, nil
}

func (h *handlers) delete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	return h.each(c, nil, func(ctx context.Context, m Message) (Message, error) { return h.b.Delete(ctx, m.ID) })
}

func (h *handlers) mailboxCreate(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	name := strings.TrimSpace(c.Args[0])
	if name == "" || strings.ContainsAny(name, "/\x00") {
		return nil, agentcli.Usagef("mailbox name must be non-empty and contain no '/'")
	}
	path := localAccount + "/" + name
	ms, err := h.b.Mailboxes(c)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Path == path {
			return nil, agentcli.Conflictf("mailbox %q already exists", path)
		}
	}
	return &agentcli.WritePlan{Scopes: []string{path}, Targets: []agentcli.Item{{"path": path}},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			m, err := h.b.CreateMailbox(ctx, name)
			if err != nil {
				return nil, err
			}
			return []agentcli.Item{{"path": m.Path, "account": m.Account, "unread": m.Unread}}, nil
		}}, nil
}

type draftInput struct {
	To      []string `json:"to"`
	CC      []string `json:"cc"`
	BCC     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	From    string   `json:"from"`
}

// draftScope is the policy scope of a draft: "draft:<sender address>", or
// "draft:default" when Mail picks the sender.
func draftScope(from string) string {
	if from == "" {
		return "draft:default"
	}
	return "draft:" + strings.ToLower(from)
}

func addresses(field string, in []string) ([]string, error) {
	var out []string
	for _, a := range in {
		p, err := netmail.ParseAddress(a)
		if err != nil {
			return nil, agentcli.Usagef("invalid %s address %q", field, a)
		}
		out = append(out, p.Address)
	}
	return out, nil
}

func (h *handlers) draft(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	var ins []draftInput
	if len(c.Args) > 0 {
		if c.Args[0] != "-" {
			return nil, agentcli.Usagef("the only positional argument is '-' (stdin)")
		}
		var err error
		if ins, err = agentcli.DecodeInputs[draftInput](c.Stdin); err != nil {
			return nil, err
		}
	} else {
		ins = []draftInput{{To: c.Strings("to"), CC: c.Strings("cc"), BCC: c.Strings("bcc"),
			Subject: c.String("subject"), Body: c.String("body"), From: c.String("from")}}
	}
	var senders map[string]bool
	var drafts []Draft
	var scopes []string
	var targets []agentcli.Item
	for i, in := range ins {
		if strings.TrimSpace(in.Subject) == "" && strings.TrimSpace(in.Body) == "" {
			return nil, agentcli.Usagef("draft %d: subject or body is required", i+1)
		}
		d := Draft{Subject: in.Subject, Body: in.Body}
		var err error
		if d.To, err = addresses("to", in.To); err != nil {
			return nil, err
		}
		if d.CC, err = addresses("cc", in.CC); err != nil {
			return nil, err
		}
		if d.BCC, err = addresses("bcc", in.BCC); err != nil {
			return nil, err
		}
		if in.From != "" {
			from, err := addresses("from", []string{in.From})
			if err != nil {
				return nil, err
			}
			if senders == nil {
				senders = map[string]bool{}
				as, err := h.b.Accounts(c)
				if err != nil {
					return nil, err
				}
				for _, a := range as {
					for _, e := range a.Emails {
						senders[strings.ToLower(e)] = true
					}
				}
			}
			if !senders[strings.ToLower(from[0])] {
				return nil, agentcli.NotFoundf("%s is not an address of any Mail account", from[0])
			}
			d.From = from[0]
		}
		drafts = append(drafts, d)
		scopes = append(scopes, draftScope(d.From))
		targets = append(targets, agentcli.Item{"from": d.From, "to": nonNil(d.To), "cc": nonNil(d.CC), "subject": d.Subject})
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, d := range drafts {
			saved, err := h.b.SaveDraft(ctx, d)
			if err != nil {
				return nil, partial(err, len(out), len(drafts))
			}
			out = append(out, agentcli.Item{"from": saved.From, "to": nonNil(d.To), "cc": nonNil(d.CC), "subject": d.Subject, "saved": true})
		}
		return out, nil
	}}, nil
}
