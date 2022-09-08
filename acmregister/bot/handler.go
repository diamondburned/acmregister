package bot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

// TODO: if member is already registered, just give them the role
// TODO: command to migrate roles

// ConfirmationEmailScheduler schedules a confirmation email to be sent and a
// follow-up to be notified to the user. To send this specific follow-up, use
// Handler.EmailSentFollowup.
type ConfirmationEmailScheduler interface {
	// ScheduleConfirmationEmail asynchronously schedules an email to be sent in
	// the background. It has no error reporting; the implementation is expected
	// to use the InteractionEvent to send a reply.
	ScheduleConfirmationEmail(c *Client, ev *discord.InteractionEvent, m acmregister.Member)
	// Close cancels any scheduled jobs, if any.
	Close() error
}

// NewAsyncConfirmationEmailSender creates a new ConfirmationEmailScheduler. Its
// job is to send an email over SMTP and deliver a response within a goroutine.
func NewAsyncConfirmationEmailSender(smtpVerifier *verifyemail.SMTPVerifier) ConfirmationEmailScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncConfirmationEmailSender{
		vr:     smtpVerifier,
		ctx:    ctx,
		cancel: cancel,
	}
}

type asyncConfirmationEmailSender struct {
	vr     *verifyemail.SMTPVerifier
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *asyncConfirmationEmailSender) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *asyncConfirmationEmailSender) ScheduleConfirmationEmail(c *Client, ev *discord.InteractionEvent, m acmregister.Member) {
	// This might take a while.
	s.wg.Add(1)
	go func() {
		log.Println("SMTP goroutine booted up")
		defer s.wg.Done()

		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()

		if err := s.vr.SendConfirmationEmail(ctx, m); err != nil {
			c.LogErr(m.GuildID, errors.Wrap(err, "cannot send confirmation email"))
			c.FollowUp(ev, InternalErrorResponseData())
		} else {
			c.FollowUp(ev, EmailSentFollowupData())
		}
	}()
}

type Opts struct {
	Store          acmregister.Store
	PINStore       verifyemail.PINStore           // optional
	EmailHosts     acmregister.EmailHostsVerifier // optional
	EmailVerifier  acmregister.EmailVerifier      // optional
	EmailScheduler ConfirmationEmailScheduler     // optional
}

func (o Opts) verifyEmail(ctx context.Context, email acmregister.Email) error {
	if err := o.EmailHosts.VerifyEmail(email); err != nil {
		return err
	}

	if o.EmailVerifier != nil {
		if err := o.EmailVerifier.VerifyEmail(ctx, email); err != nil {
			return err
		}
	}

	return nil
}

type Handler struct {
	Client
	store acmregister.Store
	opts  Opts
}

// NewHandler creates a new Handler instance bound to the given State.
func NewHandler(s *state.State, opts Opts) *Handler {
	return &Handler{
		Client: *NewClient(s.Context(), s),
		store:  opts.Store,
		opts:   opts,
	}
}

func (h *Handler) Intents() gateway.Intents {
	return 0 |
		gateway.IntentGuilds |
		gateway.IntentDirectMessages
}

func (h *Handler) HandleInteraction(ev *discord.InteractionEvent) *api.InteractionResponse {
	// Have we stopped?
	select {
	case <-h.ctx.Done():
		// If yes, bail. We're not supposed to be handling any events, so we
		// ignore everything.
		return nil
	default:
	}

	defer func() {
		if panicked := recover(); panicked != nil {
			h.PrivateWarning(ev, fmt.Errorf("bug: panic occured: %v", panicked))
		}
	}()

	switch data := ev.Data.(type) {
	case *discord.CommandInteraction:
		p, err := h.s.Permissions(ev.ChannelID, ev.SenderID())
		if err != nil {
			return ErrorResponse(errors.Wrap(err, "cannot get permission for yourself"))
		}

		// Limit all commands to admins only.
		if !p.Has(discord.PermissionAdministrator) {
			return ErrorResponse(fmt.Errorf("you're not an administrator; contact the guild owner"))
		}

		switch data.Name {
		case "init-register":
			return h.cmdInit(ev, data.Options)
		case "registered-member":
			if len(data.Options) == 1 {
				switch data := data.Options[0]; data.Name {
				case "query":
					return h.cmdMemberQuery(ev, data.Options)
				case "unregister":
					return h.cmdMemberUnregister(ev, data.Options)
				case "reset-name":
					return h.cmdMemberResetName(ev, data.Options)
				}
			}
		case "clear-registration":
			return h.cmdClear(ev, data.Options)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown command %q", data.Name)
		}
	case *discord.ButtonInteraction:
		switch data.CustomID {
		case "register":
			return h.buttonRegister(ev)
		case "verify-pin":
			return h.buttonVerifyPIN(ev)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown button %q", data.CustomID)
		}
	case *discord.ModalInteraction:
		switch data.CustomID {
		case "register-response":
			return h.modalRegisterResponse(ev, data)
		case "verify-pin":
			return h.modalVerifyPIN(ev, data)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown modal %q", data.CustomID)
		}
	case *discord.PingInteraction:
		return &api.InteractionResponse{Type: api.PongInteraction}
	default:
		logger := logger.FromContext(h.ctx)
		logger.Printf("not handling unknown command type %T", data)
	}

	return nil
}

// Client wraps around state.State for some common functionalities.
type Client struct {
	s   *state.State
	ctx context.Context
}

// NewClient creates a new Client instance.
func NewClient(ctx context.Context, s *state.State) *Client {
	return &Client{
		s:   s,
		ctx: ctx,
	}
}

// Context returns the internal context.
func (c *Client) Context() context.Context {
	return c.ctx
}

// FollowUp sends a followup response.
func (c *Client) FollowUp(ev *discord.InteractionEvent, data *api.InteractionResponseData) {
	// Try for a few seconds.
	ctx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
	defer cancel()

	s := c.s.WithContext(ctx)
	var err error

	for ctx.Err() == nil {
		_, err = s.FollowUpInteraction(ev.AppID, ev.Token, *data)
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if err != nil {
		err = errors.Wrap(err, "cannot follow-up to interaction")
		c.LogErr(ev.GuildID, err)
	}
}

// PrivateWarning is like PrivateErr, except the user does not get a reply back
// saying things have gone wrong. Use this if we don't intend to return after
// the error.
func (c *Client) PrivateWarning(ev *discord.InteractionEvent, sendErr error) {
	c.LogErr(ev.GuildID, sendErr)
}

// LogErr logs the given error to stdout. It attaches guild information if
// possible.
func (c *Client) LogErr(guildID discord.GuildID, err error) {
	var guildInfo string
	if guild, err := c.s.Guild(guildID); err == nil {
		guildInfo = fmt.Sprintf("%q (%d)", guild.Name, guild.ID)
	} else {
		guildInfo = fmt.Sprintf("%d", guildID)
	}

	logger := logger.FromContext(c.ctx)
	logger.Println("guild "+guildInfo+":", "command error:", err)
}

func msgResponse(data *api.InteractionResponseData) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: data,
	}
}

func deferResponse(flags discord.MessageFlags) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.DeferredMessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Flags: flags,
		},
	}
}

// InternalErrorResponse is used in case of confidential errors.
func InternalErrorResponse() *api.InteractionResponse {
	return ErrorResponse(errors.New("internal error occured, please contact the server administrator"))
}

// InternalErrorResponseData is used in case of confidential errors.
func InternalErrorResponseData() *api.InteractionResponseData {
	return InternalErrorResponse().Data
}

// ErrorResponse creates a new erroneous interaction response.
func ErrorResponse(err error) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: ErrorResponseData(err),
	}
}

// ErrorResponseData creates a new erroneous interaction response data.
func ErrorResponseData(err error) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content: option.NewNullableString("⚠️ **Error:** " + err.Error()),
		Flags:   discord.EphemeralMessage,
	}
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
