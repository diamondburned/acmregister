package acmregister

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/pkg/errors"
)

// ErrNotFound is returned if anything is not found.
var ErrNotFound = errors.New("not found")

// ErrUnknownPronouns is returned if a Pronouns is unknown. Use NewPronouns and
// check its error to ensure that it is never invalid.
var ErrUnknownPronouns = errors.New("unknown pronouns")

type KnownGuild struct {
	GuildID           discord.GuildID
	ChannelID         discord.ChannelID
	RoleID            discord.RoleID
	InitUserID        discord.UserID
	RegisteredMessage string
}

type Member struct {
	GuildID  discord.GuildID
	UserID   discord.UserID
	Metadata MemberMetadata
}

type MemberMetadata struct {
	Email     string   `json:"email"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Pronouns  Pronouns `json:"pronouns"`
}

// AllowedEmailHosts is a whitelist of email hosts. An empty list permits all
// emails.
var AllowedEmailDomains = []string{
	"@csu.fullerton.edu",
	"@fullerton.edu",
}

// AllowedEmailHostsLabel returns AllowedEmailDomains as a label string.
func AllowedEmailDomainsLabel() string {
	switch len(AllowedEmailDomains) {
	case 0:
		return ""
	case 1:
		return AllowedEmailDomains[0]
	case 2:
		return AllowedEmailDomains[0] + " or " + AllowedEmailDomains[1]
	default:
		return strings.Join(
			AllowedEmailDomains[:len(AllowedEmailDomains)-1], ", ") +
			", or " +
			AllowedEmailDomains[len(AllowedEmailDomains)-1]
	}
}

func (m MemberMetadata) Validate() error {
	if len(AllowedEmailDomains) > 0 {
		for _, host := range AllowedEmailDomains {
			if strings.HasSuffix(m.Email, host) {
				goto ok1
			}
		}
		return fmt.Errorf("email %q does not belong to a known domain", m.Email)
	ok1:
	}

	if err := m.Pronouns.Validate(); err != nil {
		return err
	}

	return nil
}

// Pronouns describes a pronouns string in the format (they/them).
type Pronouns string

const (
	HeHim          Pronouns = "he/him"
	SheHer         Pronouns = "she/her"
	TheyThem       Pronouns = "they/them"
	AnyPronouns    Pronouns = "any"
	HiddenPronouns Pronouns = ""
)

var KnownPronouns = []Pronouns{
	HeHim,
	SheHer,
	TheyThem,
	AnyPronouns,
	HiddenPronouns,
}

// NewPronouns creates a new Pronouns value. An error is returned if the
// pronouns are invalid.
func NewPronouns(pronouns string) (Pronouns, error) {
	p := Pronouns(pronouns)
	return p, p.Validate()
}

func (p Pronouns) Validate() error {
	for _, known := range KnownPronouns {
		if known == p {
			return nil
		}
	}
	return ErrUnknownPronouns
}

// Store describes a Store instance.
type Store interface {
	// WithContext clones Store to use a new context.
	WithContext(context.Context) Store

	// InitGuild initializes a guild.
	InitGuild(KnownGuild) error
	// GuildInfo returns the information about a registered guild.
	GuildInfo(discord.GuildID) (*KnownGuild, error)
	// DeleteGuild deletes the guild with the given ID from the registered
	// database.
	DeleteGuild(discord.GuildID) error

	// MemberInfo returns the member's info for the given user.
	MemberInfo(discord.GuildID, discord.UserID) (*MemberMetadata, error)
	// RegisterMember registers the given member into the store.
	RegisterMember(discord.GuildID, discord.UserID, MemberMetadata) error
	// UnregisterMember unregisters the given member from the store.
	UnregisterMember(discord.GuildID, discord.UserID) error

	// SaveSubmission temporarily saves a submission for 15 minutes. The
	// submission doesn't have to be valid.
	SaveSubmission(discord.GuildID, discord.UserID, MemberMetadata) error
	// RestoreSubmission returns a saved submission.
	RestoreSubmission(discord.GuildID, discord.UserID) (*MemberMetadata, error)
}
