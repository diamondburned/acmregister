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

	"github.com/Shopify/gomail"
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/arikawa/v3/discord"
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
	mailTmpl *template.Template
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
		dialer:   gomail.NewDialer(host, port, info.Email, info.Password),
		mailTmpl: mailTemplate,
		store:    store,
		info:     info,
	}, nil
}

//go:embed mailtmpl.html
var mailTemplateHTML string

var mailSubjectRe = regexp.MustCompile(`(?m)^<!-- ?SUBJECT: (.*) ?-->$`)

type mailTemplateData struct {
	acmregister.MemberMetadata
	Guild GuildInfo
	PIN   PIN
}

// GuildInfo describes a short guild info.
type GuildInfo struct {
	ID   discord.GuildID
	Name string
}

// SendConfirmationEmail sends a confirmation email to the recipient with the
// email address.
func (v *SMTPVerifier) SendConfirmationEmail(ctx context.Context, guild GuildInfo, metadata acmregister.MemberMetadata) error {
	pin, err := v.store.GeneratePIN(guild.ID, metadata.Email)
	if err != nil {
		return err
	}

	var body strings.Builder
	if err := v.mailTmpl.Execute(&body, mailTemplateData{
		MemberMetadata: metadata,
		PIN:            pin,
	}); err != nil {
		return errors.Wrap(err, "bug: cannot render email")
	}

	msg := gomail.NewMessage()
	msg.SetBody("text/html", body.String())
	msg.SetHeader("From", string(v.info.Email))
	msg.SetAddressHeader("To", string(metadata.Email), metadata.Name())

	if matches := mailSubjectRe.FindStringSubmatch(body.String()); matches != nil {
		subject := matches[1]
		msg.SetHeader("Subject", subject)
	}

	if err := v.dialer.DialAndSendCtx(ctx, msg); err != nil {
		return errors.Wrap(err, "bug: cannot send email")
	}

	return nil
}

// ValidatePIN validates the PIN code and returns the email associated with it.
// The PIN code is forgotten afterwards if it's valid.
func (v *SMTPVerifier) ValidatePIN(guildID discord.GuildID, pin PIN) (acmregister.Email, error) {
	return v.store.ValidatePIN(guildID, pin)
}
