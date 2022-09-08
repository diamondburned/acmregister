package bot

import (
	"context"
	"sync"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

// ConfirmationEmailScheduler schedules a confirmation email to be sent and a
// follow-up to be notified to the user. To send this specific follow-up, use
// Handler.EmailSentFollowup.
type ConfirmationEmailScheduler interface {
	// ScheduleConfirmationEmail asynchronously schedules an email to be sent in
	// the background. It has no error reporting; the implementation is expected
	// to use the InteractionEvent to send a reply.
	ScheduleConfirmationEmail(c *Client, ev *discord.InteractionEvent, m acmregister.Member) error
	// Close cancels any scheduled jobs, if any.
	Close() error
}

// NewAsyncConfirmationEmailSender creates a new ConfirmationEmailScheduler. Its
// job is to send an email over SMTP and deliver a response within a goroutine.
func NewAsyncConfirmationEmailSender(smtpVerifier *verifyemail.SMTPVerifier) ConfirmationEmailScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncConfirmationEmailSender{
		smtp:   smtpVerifier,
		ctx:    ctx,
		cancel: cancel,
	}
}

type asyncConfirmationEmailSender struct {
	smtp   *verifyemail.SMTPVerifier
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *asyncConfirmationEmailSender) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *asyncConfirmationEmailSender) ScheduleConfirmationEmail(c *Client, ev *discord.InteractionEvent, m acmregister.Member) error {
	s.wg.Add(1)
	go func() {
		SendConfirmationEmail(s.ctx, s.smtp, c, ev, m)
		s.wg.Done()
	}()
	return nil
}

// SendConfirmationEmail send a confirmation email then follows up to the
// interaction event.
func SendConfirmationEmail(
	ctx context.Context, smtpVerifier *verifyemail.SMTPVerifier,
	c *Client, ev *discord.InteractionEvent, m acmregister.Member) {

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := smtpVerifier.SendConfirmationEmail(ctx, m); err != nil {
		c.FollowUpInternalError(ev, errors.Wrap(err, "cannot send confirmation email"))
		return
	}

	c.FollowUp(ev, EmailSentFollowupData())
}

// EmailSentFollowupData creates an *api.InteractionResponseData to be used as a
// reply to notify the user that the email has been delivered.
func EmailSentFollowupData() *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString(verifyPINMessage),
		Components: &discord.ContainerComponents{
			&discord.ActionRowComponent{
				&discord.ButtonComponent{
					Style:    discord.PrimaryButtonStyle(),
					CustomID: "verify-pin",
					Label:    verifyPINButtonLabel,
				},
			},
		},
	}
}
