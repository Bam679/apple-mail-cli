package mail

import (
	"bytes"
	"encoding/base64"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// maxParts bounds MIME recursion so hostile messages cannot exhaust memory.
const maxParts = 200

// BodyText extracts a readable plain-text body from a raw RFC 5322 message:
// text/plain is preferred, otherwise text/html is converted. Attachments are
// skipped. It never fails hard: unparseable input yields the best effort.
func BodyText(raw []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	plain, htm := "", ""
	budget := maxParts
	walk(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Header.Get("Content-Disposition"), msg.Body, &plain, &htm, &budget, 0)
	if strings.TrimSpace(plain) != "" {
		return normalize(plain)
	}
	return normalize(HTMLToText(htm))
}

func walk(ctype, cte, disp string, body io.Reader, plain, htm *string, budget *int, depth int) {
	*budget--
	if *budget < 0 || depth > 20 {
		return
	}
	if ctype == "" {
		ctype = "text/plain; charset=us-ascii"
	}
	mt, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		mt, params = "text/plain", map[string]string{}
	}
	if d, _, _ := mime.ParseMediaType(disp); d == "attachment" {
		return
	}
	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				return
			}
			walk(p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"), p.Header.Get("Content-Disposition"), p, plain, htm, budget, depth+1)
		}
	}
	if mt != "text/plain" && mt != "text/html" {
		return
	}
	data, err := io.ReadAll(io.LimitReader(decodeCTE(cte, body), 16<<20))
	if err != nil && len(data) == 0 {
		return
	}
	text := decodeCharset(params["charset"], data)
	if mt == "text/plain" && *plain == "" {
		*plain = text
	} else if mt == "text/html" && *htm == "" {
		*htm = text
	}
}

func decodeCTE(cte string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, &stripWS{r: r})
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	}
	return r
}

// stripWS removes line breaks that base64 bodies contain.
type stripWS struct{ r io.Reader }

func (s *stripWS) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	j := 0
	for i := 0; i < n; i++ {
		if c := p[i]; c != '\r' && c != '\n' && c != ' ' && c != '\t' {
			p[j] = c
			j++
		}
	}
	return j, err
}

// cp1252 maps the 0x80–0x9F range of Windows-1252; other bytes equal Latin-1.
var cp1252 = [32]rune{'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8d, 'Ž', 0x8f,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9d, 'ž', 'Ÿ'}

func decodeCharset(cs string, b []byte) string {
	switch strings.ToLower(strings.TrimSpace(cs)) {
	case "iso-8859-1", "latin1", "iso-8859-15", "windows-1252", "cp1252":
		var sb strings.Builder
		for _, c := range b {
			if c >= 0x80 && c <= 0x9f {
				sb.WriteRune(cp1252[c-0x80])
			} else {
				sb.WriteRune(rune(c))
			}
		}
		return sb.String()
	}
	return strings.ToValidUTF8(string(b), "\ufffd")
}

// HeaderDecoder decodes RFC 2047 words, including Latin-1/CP1252 charsets.
var HeaderDecoder = &mime.WordDecoder{CharsetReader: func(cs string, r io.Reader) (io.Reader, error) {
	b, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return nil, err
	}
	return strings.NewReader(decodeCharset(cs, b)), nil
}}

// HTMLToText converts HTML to readable text: it drops scripts, styles and
// hidden or tracking content, keeps link targets and turns blocks into lines.
func HTMLToText(s string) string {
	var out strings.Builder
	skip := "" // tag whose content is being skipped
	href := "" // current link target
	label := 0 // out.Len() at link start
	for len(s) > 0 {
		lt := strings.IndexByte(s, '<')
		if lt < 0 {
			if skip == "" {
				out.WriteString(html.UnescapeString(s))
			}
			break
		}
		if skip == "" {
			out.WriteString(html.UnescapeString(s[:lt]))
		}
		s = s[lt:]
		if strings.HasPrefix(s, "<!--") {
			end := strings.Index(s, "-->")
			if end < 0 {
				break
			}
			s = s[end+3:]
			continue
		}
		gt := strings.IndexByte(s, '>')
		if gt < 0 {
			break
		}
		tag := s[1:gt]
		s = s[gt+1:]
		closing := strings.HasPrefix(tag, "/")
		name := ""
		if f := strings.Fields(strings.TrimPrefix(tag, "/")); len(f) > 0 {
			name = strings.TrimSuffix(strings.ToLower(f[0]), "/")
		}
		if skip != "" {
			if closing && name == skip {
				skip = ""
			}
			continue
		}
		lower := strings.ToLower(tag)
		switch {
		case !closing && (name == "script" || name == "style" || name == "head" || name == "title"):
			skip = name
		case !closing && hidden(lower):
			if !strings.HasSuffix(strings.TrimSpace(tag), "/") && name != "img" && name != "br" {
				skip = name
			}
		case name == "br":
			out.WriteString("\n")
		case name == "li" && !closing:
			out.WriteString("\n- ")
		case name == "a" && !closing:
			href, label = attr(tag, "href"), out.Len()
		case name == "a" && closing:
			text := out.String()[label:]
			// Bounded check keeps pathological link-heavy input linear.
			if href != "" && !strings.HasPrefix(href, "mailto:") && (len(text) > 4096 || !strings.Contains(text, href)) {
				out.WriteString(" (" + href + ")")
			}
			href = ""
		case isBlock(name):
			out.WriteString("\n")
		}
	}
	return out.String()
}

func hidden(tag string) bool {
	t := strings.ReplaceAll(tag, " ", "")
	return strings.Contains(t, "display:none") || strings.Contains(t, "mso-hide:all") ||
		strings.Contains(t, `aria-hidden="true"`) || strings.Contains(t, "width=\"1\"") && strings.HasPrefix(t, "img")
}

func isBlock(name string) bool {
	switch name {
	case "p", "div", "tr", "table", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "ul", "ol", "hr", "section", "article", "header", "footer":
		return true
	}
	return false
}

func attr(tag, key string) string {
	lower := strings.ToLower(tag)
	i := strings.Index(lower, key+"=")
	if i < 0 {
		return ""
	}
	v := tag[i+len(key)+1:]
	if v == "" {
		return ""
	}
	if q := v[0]; q == '"' || q == '\'' {
		if end := strings.IndexByte(v[1:], q); end >= 0 {
			return html.UnescapeString(v[1 : end+1])
		}
		return ""
	}
	if end := strings.IndexAny(v, " \t>"); end >= 0 {
		v = v[:end]
	}
	return html.UnescapeString(v)
}

// normalize trims trailing spaces, removes zero-width characters and
// collapses runs of more than one blank line.
func normalize(s string) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "", "\u00ad", "", "\u00a0", " ").Replace(s)
	lines := strings.Split(s, "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(strings.Join(strings.Fields(l), " "), " ")
		if l == "" {
			blank++
			if blank > 1 || len(out) == 0 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	res := strings.TrimSpace(strings.Join(out, "\n"))
	if !utf8.ValidString(res) {
		res = strings.ToValidUTF8(res, "\ufffd")
	}
	return res
}
