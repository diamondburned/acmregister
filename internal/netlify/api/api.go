// package api is when we reinvent RPC but because AWS Lambda sucks balls. This
// package is the intermediary package that binds all Netlify functions
// together.
package api

import (
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/arikawa/v3/discord"
)

type VerifyEmailData struct {
	AppID  discord.AppID      `json:"app_id"`
	Token  string             `json:"token"`
	Member acmregister.Member `json:"member"`
}
