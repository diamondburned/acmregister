package verifyemail

import (
	"context"
	"testing"

	"github.com/diamondburned/acmregister/acmregister"
)

func TestShibbolethVerifier(t *testing.T) {
	ctx := context.Background()

	verifier := ShibbolethVerifier{
		URL: "https://my.fullerton.edu",
	}

	check := func(email acmregister.Email, valid bool) {
		err := verifier.VerifyEmail(ctx, email)
		if valid && err != nil {
			t.Fatalf("%s: %v", email, err)
		}
		if !valid && err == nil {
			t.Fatalf("%s: expected invalid email", email)
		}
	}

	check("diamondburned@csu.fullerton.edu", true)
	check("aaronlieberman@csu.fullerton.edu", true)
	check("pinventado@fullerton.edu", true)
	check("ioghrsoughrsiogsg@fullerton.edu", false)
	check("iunfheiuhfneihfne@csu.fulllerton.edu", false)
}
