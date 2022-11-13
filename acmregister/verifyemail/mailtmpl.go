package verifyemail

import (
	"html/template"
	"strings"

	_ "embed"

	"github.com/diamondburned/html2text"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

//go:embed mailtmpl.html
var mailTemplateHTML string

type mailTemplate struct {
	html *template.Template
}

type renderedMail struct {
	Subject  string
	HTMLBody string
	TextBody string
}

var html2textOptions = html2text.Options{
	PrettyTables:        true,
	PrettyTablesOptions: html2text.NewPrettyTablesOptions(),
}

func parseMailTemplate(html string) (*mailTemplate, error) {
	if strings.Contains(html, "\n") && !strings.Contains(html, "\r") {
		// Convert LF to CRLF to satisfy the email spec.
		html = unix2dos(html)
	}

	t, err := template.New("").Parse(html)
	if err != nil {
		return nil, errors.Wrap(err, "template error")
	}

	_, err = html2text.FromString(html, html2textOptions)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert HTML to text")
	}

	return &mailTemplate{html: t}, nil
}

func (t *mailTemplate) Render(v any) (*renderedMail, error) {
	var b strings.Builder
	if err := t.html.Execute(&b, v); err != nil {
		return nil, errors.Wrap(err, "cannot render mail template")
	}
	html := b.String()
	subject := extractSubject(html)

	text, err := html2text.FromString(html, html2textOptions)
	if err != nil {
		text = html // fallback
	} else {
		text = unix2dos(text)
	}

	return &renderedMail{
		Subject:  subject,
		HTMLBody: html,
		TextBody: text,
	}, nil
}

func extractSubject(doc string) string {
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return ""
	}

	e := &subjectExtractor{}
	e.walk(node)

	return e.subject
}

type subjectExtractor struct {
	subject string
}

func (e *subjectExtractor) walk(n *html.Node) {
	switch n.Type {
	case html.ElementNode:
		switch n.Data {
		case "title":
			e.subject = n.FirstChild.Data
			return
		case "meta":
			if len(n.Attr) == 2 {
				if n.Attr[0].Key == "name" && n.Attr[0].Val == "email:subject" {
					e.subject = n.Attr[1].Val
					return
				}
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.walk(c)
	}
}

func unix2dos(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}
