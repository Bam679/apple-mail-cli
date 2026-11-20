// Package mail implements the apple-mail CLI on top of a Bridge to Mail.app
// (JXA in production, a fake in tests). There is intentionally no way to
// send, draft or forward mail.
package mail

import (
	"context"
	_ "embed"
	"time"

	"github.com/Bam679/apple-mail-cli/internal/osa"
)

// Account is a Mail account.
type Account struct {
	Name    string   `json:"name"`
	Emails  []string `json:"emails"`
	Enabled bool     `json:"enabled"`
}

// Mailbox is a mailbox addressed by its path "Account/Box/Sub" (local
// mailboxes use the account "On My Mac").
type Mailbox struct {
	Path    string `json:"path"`
	Account string `json:"account"`
	Unread  int    `json:"unread"`
}

// Attachment describes an attachment (content is never fetched).
type Attachment struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	MIME string `json:"mime"`
}

// Message is a message summary; Get fills the detail fields.
type Message struct {
	ID            string
	Subject       string
	From          string
	Date          time.Time
	Read          bool
	Flagged       bool
	Mailbox       string
	MessageID     string
	To            []string
	CC            []string
	DateSent      time.Time
	Size          int64
	Junk          bool
	Attachments   []Attachment
	Source        string
	SourceSkipped string
}

// Query selects messages across mailboxes, newest first.
type Query struct {
	Paths       []string
	Offset      int
	Limit       int
	Unread      bool
	Flagged     bool
	Since       time.Time
	Before      time.Time
	Text        string
	SubjectOnly bool
}

// Page is one page of messages plus the total number of matches.
type Page struct {
	Total int
	Items []Message
}

// Bridge is the OS boundary. Implementations must not interpret input as code.
type Bridge interface {
	Accounts(ctx context.Context) ([]Account, error)
	Mailboxes(ctx context.Context) ([]Mailbox, error)
	Inboxes(ctx context.Context) ([]string, error)
	Messages(ctx context.Context, q Query) (Page, error)
	Get(ctx context.Context, id string, withSource bool) (Message, error)
	Mark(ctx context.Context, id string, read, flagged *bool) (Message, error)
	Move(ctx context.Context, id, to string) (Message, error)
	Delete(ctx context.Context, id string) (Message, error)
	CreateMailbox(ctx context.Context, name string) (Mailbox, error)
}

// MaxSource is the largest raw message fetched for body extraction.
const MaxSource = 10 << 20

//go:embed mail.js
var script string

// JXA is the production Bridge backed by Mail.app through osascript.
type JXA struct {
	Runner osa.Runner
}

// NewJXA returns the production bridge.
func NewJXA() *JXA { return &JXA{Runner: osa.Exec{}} }

type wireMessage struct {
	ID            string       `json:"id"`
	Subject       string       `json:"subject"`
	From          string       `json:"from"`
	DateMS        *int64       `json:"date_ms"`
	Read          bool         `json:"read"`
	Flagged       bool         `json:"flagged"`
	Mailbox       string       `json:"mailbox"`
	MessageID     string       `json:"message_id"`
	To            []string     `json:"to"`
	CC            []string     `json:"cc"`
	DateSentMS    *int64       `json:"date_sent_ms"`
	Size          int64        `json:"size"`
	Junk          bool         `json:"junk"`
	Attachments   []Attachment `json:"attachments"`
	Source        string       `json:"source"`
	SourceSkipped string       `json:"source_skipped"`
}

func fromMS(v *int64) time.Time {
	if v == nil {
		return time.Time{}
	}
	return time.UnixMilli(*v).Local()
}

func msOf(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func (w wireMessage) message() Message {
	return Message{ID: w.ID, Subject: w.Subject, From: w.From, Date: fromMS(w.DateMS), Read: w.Read, Flagged: w.Flagged,
		Mailbox: w.Mailbox, MessageID: w.MessageID, To: w.To, CC: w.CC, DateSent: fromMS(w.DateSentMS), Size: w.Size,
		Junk: w.Junk, Attachments: w.Attachments, Source: w.Source, SourceSkipped: w.SourceSkipped}
}

func (j *JXA) one(ctx context.Context, op string, args any) (Message, error) {
	var w wireMessage
	if err := j.Runner.Run(ctx, script, op, args, &w); err != nil {
		return Message{}, err
	}
	return w.message(), nil
}

// Accounts implements Bridge.
func (j *JXA) Accounts(ctx context.Context) ([]Account, error) {
	var out []Account
	err := j.Runner.Run(ctx, script, "accounts", nil, &out)
	return out, err
}

// Mailboxes implements Bridge.
func (j *JXA) Mailboxes(ctx context.Context) ([]Mailbox, error) {
	var out []Mailbox
	err := j.Runner.Run(ctx, script, "mailboxes", nil, &out)
	return out, err
}

// Inboxes implements Bridge.
func (j *JXA) Inboxes(ctx context.Context) ([]string, error) {
	var out []string
	err := j.Runner.Run(ctx, script, "inboxes", nil, &out)
	return out, err
}

// Messages implements Bridge.
func (j *JXA) Messages(ctx context.Context, q Query) (Page, error) {
	var w struct {
		Total int           `json:"total"`
		Items []wireMessage `json:"items"`
	}
	args := map[string]any{"paths": q.Paths, "offset": q.Offset, "limit": q.Limit, "unread": q.Unread, "flagged": q.Flagged,
		"since_ms": msOf(q.Since), "before_ms": msOf(q.Before), "query": q.Text, "subject_only": q.SubjectOnly}
	if err := j.Runner.Run(ctx, script, "messages", args, &w); err != nil {
		return Page{}, err
	}
	p := Page{Total: w.Total}
	for _, m := range w.Items {
		p.Items = append(p.Items, m.message())
	}
	return p, nil
}

// Get implements Bridge.
func (j *JXA) Get(ctx context.Context, id string, withSource bool) (Message, error) {
	return j.one(ctx, "get", map[string]any{"id": id, "source": withSource, "max_source": MaxSource})
}

// Mark implements Bridge.
func (j *JXA) Mark(ctx context.Context, id string, read, flagged *bool) (Message, error) {
	return j.one(ctx, "mark", map[string]any{"id": id, "read": read, "flagged": flagged})
}

// Move implements Bridge.
func (j *JXA) Move(ctx context.Context, id, to string) (Message, error) {
	return j.one(ctx, "move", map[string]string{"id": id, "to": to})
}

// Delete implements Bridge.
func (j *JXA) Delete(ctx context.Context, id string) (Message, error) {
	return j.one(ctx, "delete", map[string]string{"id": id})
}

// CreateMailbox implements Bridge.
func (j *JXA) CreateMailbox(ctx context.Context, name string) (Mailbox, error) {
	var m Mailbox
	err := j.Runner.Run(ctx, script, "mailbox_create", map[string]string{"name": name}, &m)
	return m, err
}
