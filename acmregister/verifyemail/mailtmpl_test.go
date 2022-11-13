package verifyemail

import (
	"strings"
	"testing"
)

func TestMailTemplate(t *testing.T) {
	sampleHTML := trimLFHead(`
<meta name="email:subject" content="Hello, {{ . }}!">
<h1>Hello, {{ . }}!</h1>
<p>How are you?</p>
`)

	tmpl, err := parseMailTemplate(sampleHTML)
	if err != nil {
		t.Fatal(err)
	}

	data, err := tmpl.Render("世界")
	if err != nil {
		t.Fatal("cannot render mail template:", err)
	}

	expectedData := &renderedMail{
		Subject: unix2dos("Hello, 世界!"),
		HTMLBody: unix2dos(trimLFHead(`
<meta name="email:subject" content="Hello, 世界!">
<h1>Hello, 世界!</h1>
<p>How are you?</p>
`)),
		TextBody: unix2dos(trimLFHead(`
************
Hello, 世界!
************

How are you?`)),
	}

	if data.Subject != expectedData.Subject {
		t.Errorf("unexpected subject\n"+
			"expected: %q\n"+
			"actual:   %q",
			expectedData.Subject, data.Subject)
	}

	if data.HTMLBody != expectedData.HTMLBody {
		t.Errorf("unexpected HTML body\n"+
			"expected: %q\n"+
			"actual:   %q",
			expectedData.HTMLBody, data.HTMLBody)
	}

	if data.TextBody != expectedData.TextBody {
		t.Errorf("unexpected text body\n"+
			"expected: %q\n"+
			"actual:   %q",
			expectedData.TextBody, data.TextBody)
	}
}

func trimLFHead(s string) string {
	return strings.TrimPrefix(s, "\n")
}
