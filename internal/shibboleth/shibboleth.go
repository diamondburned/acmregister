package shibboleth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
)

func IsValidUser(ctx context.Context, uri, username string) (bool, error) {
	if username == "" {
		return false, errors.New("empty username given")
	}

	jar, _ := cookiejar.New(nil)
	client := client{
		Client: &http.Client{
			Jar: jar,
			Transport: wrapTransport(nil, func(r *http.Request, rt http.RoundTripper) (*http.Response, error) {
				r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0")
				r.Header.Set("Origin", "https://shibboleth.fullerton.edu")
				return rt.RoundTrip(r)
			}),
		},
		ctx: ctx,
	}

	shibbolethURL, err := client.followRedirect(uri)
	if err != nil {
		return false, errors.Wrap(err, "cannot follow redirect")
	}

	errorReq, err := client.PostForm(shibbolethURL.String(), url.Values{
		"j_username":       {username},
		"j_password":       {"a"},
		"_eventId_proceed": {""},
	})
	if err != nil {
		return false, errors.Wrap(err, "cannot test logging in")
	}
	defer errorReq.Body.Close()

	errorDoc, err := goquery.NewDocumentFromReader(errorReq.Body)
	if err != nil {
		return false, errors.Wrap(err, "cannot parse logging in HTML body")
	}
	errorReq.Body.Close()

	loginErrorElem := errorDoc.Find(`form[name="loginForm"] p.form-error`).First()
	if loginErrorElem == nil {
		return false, errors.Wrap(err, "cannot find p.form-error")
	}

	switch loginErr := strings.TrimSpace(loginErrorElem.Text()); loginErr {
	case "The password you entered was incorrect.":
		return true, nil
	case "The username you entered cannot be identified.":
		return false, nil
	default:
		return false, fmt.Errorf("unknown login error: %q", loginErr)
	}
}

type client struct {
	*http.Client
	ctx context.Context
}

func (c *client) followRedirect(uri string) (*url.URL, error) {
	r, err := c.Get(uri)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	return r.Request.URL, nil
}

// Bump execution: e1s1 -> e1s2
func bumpExecution(u *url.URL) *url.URL {
	q := u.Query()

	execution := q.Get("execution")
	if !strings.HasPrefix(execution, "e1s") {
		return u
	}

	s, err := strconv.Atoi(strings.TrimPrefix(execution, "e1s"))
	if err != nil {
		return u
	}

	s++
	q.Set("execution", fmt.Sprintf("e1s%d", s))

	cpy := *u
	cpy.RawQuery = q.Encode()

	return &cpy
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func wrapTransport(old http.RoundTripper, f func(*http.Request, http.RoundTripper) (*http.Response, error)) http.RoundTripper {
	if old == nil {
		old = http.DefaultTransport
	}
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return f(r, old)
	})
}
