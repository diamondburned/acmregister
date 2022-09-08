package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/internal/netlify/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/pkg/errors"
)

type confirmationEmailScheduler struct {
	url string
	ctx context.Context
}

var asyncClient = http.Client{
	Transport: &http.Transport{
		DisableCompression: true,
		// Immediately expire this. We don't care about waiting for the server.
		ResponseHeaderTimeout: 1 * time.Nanosecond,
	},
}

func (s confirmationEmailScheduler) Close() error {
	return nil
}

func (s confirmationEmailScheduler) ScheduleConfirmationEmail(c *bot.Client, ev *discord.InteractionEvent, m acmregister.Member) {
	body, err := json.Marshal(api.VerifyEmailData{
		AppID:  ev.AppID,
		Token:  ev.Token,
		Member: m,
	})
	if err != nil {
		c.FollowUpInternalError(ev, errors.Wrap(err, "cannot marshal VerifyEmailData"))
		return
	}

	req, err := http.NewRequestWithContext(s.ctx,
		"POST", s.url+"/.netlify/functions/verifyemail", bytes.NewReader(body))
	if err != nil {
		c.FollowUpInternalError(ev, errors.Wrap(err, "cannot create request to /verifyemail"))
		return
	}

	req.Header.Set("Content-Type", "encoding/json")

	resp, err := asyncClient.Do(req)
	if err != nil {
		// Expect this error, since we shot ourselves in the foor with
		// asyncClient.
		expected := strings.HasSuffix(err.Error(), "timeout awaiting response headers")

		if !expected {
			c.FollowUpInternalError(ev, errors.Wrap(err, "cannot POST to /verifyemail"))
			return
		}
	} else {
		// We don't even bother waiting for the request to finish. Just close it
		// early.
		resp.Body.Close()
	}
}
