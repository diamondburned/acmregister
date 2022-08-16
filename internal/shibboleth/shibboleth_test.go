package shibboleth

import (
	"context"
	"testing"
)

func TestIsValidUser(t *testing.T) {
	ctx := context.Background()
	const domain = "https://my.fullerton.edu"

	check := func(username string, valid bool) {
		b, err := IsValidUser(ctx, domain, username)
		if err != nil {
			t.Fatalf("%s: %v", username, err)
		}
		if b != valid {
			t.Errorf("%s: expected valid = %v, got %v", username, valid, b)
		}
	}

	check("diamondburned", true)
	check("aaronlieberman", true)
	check("fjiejfioejgrsioghrsoughrsiogsg", false)
}
