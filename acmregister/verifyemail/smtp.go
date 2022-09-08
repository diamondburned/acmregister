package verifyemail

import (
	"context"
	"html/template"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "embed"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/gomail"
	"github.com/pkg/errors"
)

type SMTPInfo struct {
	Host         string
	Email        string
	Password     string
	TemplatePath string
}

type SMTPVerifier struct {
	PINStore
	dialer   *gomail.Dialer
	mailTmpl *template.Template
	info     SMTPInfo
}

func NewSMTPVerifier(info SMTPInfo, pinStore PINStore) (*SMTPVerifier, error) {
	if !strings.Contains(info.Host, ":") {
		info.Host += ":465"
	}

	mailTemplateHTML := mailTemplateHTML
	if info.TemplatePath != "" {
		b, err := os.ReadFile(info.TemplatePath)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read mail template path")
		}
		mailTemplateHTML = string(b)
	}

	mailTemplate, err := template.New("").Parse(
		strings.ReplaceAll(mailTemplateHTML, "\n", "\r\n"))
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse mail template HTML")
	}

	host, portStr, err := net.SplitHostPort(info.Host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid info.Host")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid port in info.Host")
	}

	return &SMTPVerifier{
		PINStore: pinStore,

		dialer:   gomail.NewDialer(host, port, info.Email, info.Password),
		mailTmpl: mailTemplate,
		info:     info,
	}, nil
}

//go:embed mailtmpl.html
var mailTemplateHTML string

var mailSubjectRe = regexp.MustCompile(`(?m)^<!-- ?SUBJECT: (.*) ?-->$`)

type mailTemplateData struct {
	acmregister.MemberMetadata
	PIN PIN
}

// SendConfirmationEmail sends a confirmation email to the recipient with the
// email address.
func (v *SMTPVerifier) SendConfirmationEmail(ctx context.Context, metadata acmregister.MemberMetadata, pin PIN) error {
	log := logger.FromContext(ctx)
	log.Println("generating mail template body...")

	var body strings.Builder
	if err := v.mailTmpl.Execute(&body, mailTemplateData{
		MemberMetadata: metadata,
		PIN:            pin,
	}); err != nil {
		return errors.Wrap(err, "bug: cannot render email")
	}

	log.Println("creating mail...")

	msg := gomail.NewMessage(gomail.SetContext(ctx))
	msg.SetBody("text/html", body.String())
	msg.SetHeader("From", string(v.info.Email))
	msg.SetAddressHeader("To", string(metadata.Email), metadata.Name())

	if matches := mailSubjectRe.FindStringSubmatch(body.String()); matches != nil {
		subject := matches[1]
		msg.SetHeader("Subject", subject)
	}

	log.Println("dialing SMTP")

	s, err := v.dialer.DialCtx(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot dial SMTP")
	}
	defer s.Close()

	log.Println("SMTP dialed, sending mail...")

	if err := gomail.Send(s, msg); err != nil {
		return errors.Wrap(err, "cannot send SMTP email")
	}

	log.Println("SMTP dialed and mail sent")

	return nil
}
