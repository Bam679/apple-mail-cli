package mail

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBodyTextPlainPreferred(t *testing.T) {
	raw := "From: a@example.com\r\nSubject: x\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=B\r\n\r\n" +
		"--B\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nHall=C3=B6 Welt=\r\n!\r\n" +
		"--B\r\nContent-Type: text/html\r\n\r\n<p>ignored</p>\r\n--B--\r\n"
	if got := BodyText([]byte(raw)); got != "Hallö Welt!" {
		t.Fatalf("got %q", got)
	}
}

func TestBodyTextHTMLFallback(t *testing.T) {
	raw := "Content-Type: multipart/mixed; boundary=X\r\n\r\n--X\r\nContent-Type: text/html; charset=iso-8859-1\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		"PGh0bWw+PGhlYWQ+PHRpdGxlPlQ8L3RpdGxlPjwvaGVhZD48Ym9keT48cD5HcvzfZTwvcD48L2JvZHk+PC9odG1sPg==\r\n" +
		"--X\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=a.pdf\r\n\r\nJVBERi0=\r\n--X--\r\n"
	if got := BodyText([]byte(raw)); got != "Grüße" {
		t.Fatalf("got %q", got)
	}
}

func TestHTMLToText(t *testing.T) {
	in := `<html><head><style>p{}</style></head><body><script>evil()</script><!-- c -->
<h1>Title</h1><p>Hello&nbsp;<b>you</b> &amp; me</p><ul><li>one</li><li>two</li></ul>
<a href="https://example.com/x">click</a> <a href="mailto:a@b.c">a@b.c</a>
<div style="display: none">hidden text</div><span aria-hidden="true">aria</span>
<img src="t.gif" width="1" height="1">line<br>break &#8203;zero</body></html>`
	got := normalize(HTMLToText(in))
	want := "Title\n\nHello you & me\n\n- one\n- two\n\nclick (https://example.com/x) a@b.c\n\nline\nbreak zero"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBodyTextEdgeCases(t *testing.T) {
	if BodyText([]byte("garbage without headers")) != "" {
		t.Fatal("garbage")
	}
	if got := BodyText([]byte("Subject: x\r\n\r\nplain body\r\n\r\n\r\n\r\nend")); got != "plain body\n\nend" {
		t.Fatalf("default text/plain: %q", got)
	}
	if got := BodyText([]byte("Content-Type: text/plain; charset=windows-1252\r\n\r\n\x80 \x93q\x94")); got != "€ “q”" {
		t.Fatalf("cp1252: %q", got)
	}
	if got := BodyText([]byte("Content-Type: ;;bad\r\n\r\nok")); got != "ok" {
		t.Fatalf("bad content type: %q", got)
	}
	if got := BodyText([]byte("Content-Type: image/png\r\n\r\nxx")); got != "" {
		t.Fatalf("non-text: %q", got)
	}
	deep := "Content-Type: multipart/mixed; boundary=A\r\n\r\n" + strings.Repeat("--A\r\nContent-Type: text/plain\r\n\r\nx\r\n", 300) + "--A--"
	if got := BodyText([]byte(deep)); got != "x" {
		t.Fatalf("many parts: %q", got)
	}
	if attr(`a href=`, "href") != "" || attr(`a href="x`, "href") != "" || attr(`a href=x y`, "href") != "x" {
		t.Fatal("attr")
	}
	if HTMLToText("<p unclosed") != "" || HTMLToText("a<!-- open") != "a" || HTMLToText("<>x") != "x" {
		t.Fatal("broken html")
	}
}

func TestHeaderDecoder(t *testing.T) {
	got, err := HeaderDecoder.DecodeHeader("=?iso-8859-1?q?Gr=FC=DFe?= =?utf-8?b?w6Q=?=")
	if err != nil || got != "Grüßeä" {
		t.Fatalf("%q %v", got, err)
	}
}

func FuzzBodyText(f *testing.F) {
	f.Add([]byte("Content-Type: text/html\r\n\r\n<p>x<a href='y'>z</a>"))
	f.Add([]byte("Content-Type: multipart/mixed; boundary=a\r\n\r\n--a\r\n\r\nx\r\n--a--"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		out := BodyText(raw)
		if !utf8.ValidString(out) {
			t.Fatal("invalid utf8")
		}
	})
}

func FuzzHTMLToText(f *testing.F) {
	f.Add(`<a href="x">y</a><script>z</script>`)
	f.Fuzz(func(t *testing.T, s string) {
		out := HTMLToText(s)
		if strings.Contains(out, "<script") && !strings.Contains(s, "&lt;script") {
			t.Fatalf("script tag survived: %q", out)
		}
	})
}
