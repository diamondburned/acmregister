package verifyemail

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/arikawa/v3/discord"
)

// PINStore describes an interface that stores the state for verifying PINs over
// email.
type PINStore interface {
	acmregister.ContainsContext

	// GeneratePIN generates a new PIN that's assigned to the given email.
	//
	// TODO: invalidate the old PIN if there's already an existing one.
	GeneratePIN(discord.GuildID, acmregister.Email) (PIN, error)
	// ValidatePIN validates the email associated with the given PIN.
	ValidatePIN(discord.GuildID, PIN) (acmregister.Email, error)
}

// PINDigits is the number of digits in the PIN code.
const PINDigits = 4

const pinf = "%04d"

// PIN describes a PIN code.
type PIN int

// InvalidPIN is an invalid PIN to be used as a placeholder.
const InvalidPIN PIN = 0000

// String formats a PIN code from 0001 to 9999. Anything else is invalid.
func (pin PIN) String() string {
	if pin <= InvalidPIN || pin > 9999 {
		return "<invalid PIN>"
	}
	return fmt.Sprintf(pinf, pin)
}

// Format always formats the PIN as a 4-digit number without checking.
func (pin PIN) Format() string {
	return fmt.Sprintf(pinf, pin)
}

var maxPIN = int(math.Pow10(PINDigits) - 1)

// GeneratePIN generates a random PIN code for use.
func GeneratePIN() PIN {
	var pin PIN
	for pin == InvalidPIN {
		// Randomize until a valid PIN.
		pin = PIN(rand.Intn(maxPIN))
	}
	return pin
}