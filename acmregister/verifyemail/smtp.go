package verifyemail

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"

	_ "embed"

	"github.com/diamondburned/acmregister/acmregister"
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
	dialer   *gomail.Dialer
	mailTmpl *mailTemplate
	store    PINStore
	info     SMTPInfo
}

func NewSMTPVerifier(info SMTPInfo, store PINStore) (*SMTPVerifier, error) {
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

	mailTemplate, err := parseMailTemplate(mailTemplateHTML)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse mail template")
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
		dialer:   gomail.NewDialer(host, port, info.Email, info.Password),
		mailTmpl: mailTemplate,
		store:    store,
		info:     info,
	}, nil
}

type mailTemplateData struct {
	acmregister.MemberMetadata
	PIN PIN
}

// SendConfirmationEmail sends a confirmation email to the recipient with the
// email address.
func (v *SMTPVerifier) SendConfirmationEmail(ctx context.Context, member acmregister.Member) error {
	pin, err := v.store.GeneratePIN(member.GuildID, member.UserID)
	if err != nil {
		return errors.Wrap(err, "cannot generate PIN")
	}

	mailData, err := v.mailTmpl.Render(mailTemplateData{
		MemberMetadata: member.Metadata,
		PIN:            pin,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render mail")
	}

	msg := gomail.NewMessage(gomail.SetContext(ctx))
	msg.SetBody("text/plain", mailData.TextBody)
	msg.AddAlternative("text/html", mailData.HTMLBody)
	msg.SetHeader("Subject", mailData.Subject)
	msg.SetHeader("From", string(v.info.Email))
	msg.SetAddressHeader("To", string(member.Metadata.Email), member.Metadata.Name())

	s, err := v.dialer.DialCtx(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot dial SMTP")
	}
	defer s.Close()

	if err := gomail.Send(s, msg); err != nil {
		return errors.Wrap(err, "cannot send SMTP email")
	}

	return nil
}
