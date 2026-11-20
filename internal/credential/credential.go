// Package credential finds credentials for the URLs that barge reads charts
// from and writes charts to, looking in the places that other tools already
// keep them so that users rarely have to pass them to barge explicitly.
package credential

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
)

// Location is whether a URL is being read from or written to. Environment
// variables are named after it, so that a source and a destination of the
// same scheme can have different credentials.
type Location string

const (
	Source      Location = "source"
	Destination Location = "destination"
)

// Credential is what to authenticate with: a bearer Token if it's set, or else
// a Username and Password for basic authentication.
type Credential struct {
	Username string
	Password string
	// Token is a bearer token. It takes precedence over Username and Password.
	Token string
	// RefreshToken is an OAuth2 refresh token that OCI registries exchange for
	// access tokens, e.g. Azure Container Registry's. Only OCI uses it.
	RefreshToken string
}

// SetAuth sets req's Authorization header from c.
func (c *Credential) SetAuth(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	} else {
		req.SetBasicAuth(c.Username, c.Password)
	}
}

// Request describes the URL to find credentials for.
type Request struct {
	// URL is the URL to find credentials for, without userinfo.
	URL *url.URL
	// Location is whether URL is being read from or written to.
	Location Location
	// Schemes are the names that URL's scheme goes by, e.g. the alias that
	// the user wrote followed by its canonical name, most specific first.
	Schemes []string
}

// Provider finds credentials for a Request from one place. It may implement
// fmt.Stringer to describe where it looks in logs, which otherwise name its
// type.
type Provider interface {
	// Credential returns the credentials for req, or nil if it has none.
	Credential(ctx context.Context, req *Request) (*Credential, error)
}

// Chain is a list of Providers that are tried in order.
type Chain []Provider

// Credential returns credentials from the first Provider in the Chain that
// has any. A Provider that errors, e.g. on a malformed config file, is
// skipped so that it can't stop barge from trying the rest.
func (c Chain) Credential(ctx context.Context, req *Request) *Credential {
	log := util.SloggerFrom(ctx).With("host", req.URL.Host, "location", req.Location)

	for _, p := range c {
		cred, err := p.Credential(ctx, req)
		if err != nil {
			log.Debug("skipping credential provider", "provider", providerName(p), "err", err)
			continue
		}

		if cred != nil {
			log.Debug("using credentials", "provider", providerName(p))
			return cred
		}
	}

	return nil
}

// providerName describes p for logging: by its String method if it has one,
// or else by its type, rather than by its fields, which could be anything.
func providerName(p Provider) string {
	if s, ok := p.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%T", p)
}

// Default is the Chain that ForURL uses.
var Default = Chain{
	Env{},
	Cloud{},
	HelmRepositories{
		Path: func() string {
			return barge.HelmSettings().RepositoryConfig
		},
	},
	HelmRegistry{
		Path: func() string {
			return barge.HelmSettings().RegistryConfig
		},
	},
	Netrc{},
	GitHub{},
	Docker{},
}

// rejectedError marks an error as the server rejecting credentials, e.g. with
// an HTTP 401.
type rejectedError struct{ err error }

func (e *rejectedError) Error() string { return "credentials rejected: " + e.err.Error() }
func (e *rejectedError) Unwrap() error { return e.err }

// Rejected marks err as the server rejecting the credentials that it was
// given, so that Try moves on to the next credentials. Try only recognizes
// the error that Rejected returns, not errors that wrap it, so that a
// rejection from somewhere else, e.g. a chart on another host, can't make Try
// move on from credentials that worked.
func Rejected(err error) error {
	return &rejectedError{err}
}

func isRejected(err error) bool {
	_, ok := err.(*rejectedError)
	return ok
}

// Try calls fn with each of the Chain's credentials for req in turn, most
// preferred first, until fn returns anything other than a rejection from
// Rejected, and returns that. Each Provider is only asked for credentials
// once fn rejects the previous ones, and credentials that a previous Provider
// already gave are skipped. If no Provider has any credentials, Try calls fn
// once with nil. If every Provider's credentials are rejected, Try returns
// the last rejection.
//
// Env's credentials are final: if they're rejected, Try returns the rejection
// rather than trying other Providers' credentials, as the user asked for those
// credentials in particular, and other Providers may well have credentials
// for every host of the scheme that a malicious host could collect by
// rejecting them in turn.
func (c Chain) Try(ctx context.Context, req *Request, fn func(*Credential) error) error {
	log := util.SloggerFrom(ctx).With("host", req.URL.Host, "location", req.Location)

	var (
		tried    = map[Credential]bool{}
		rejected error
	)

	for _, p := range c {
		cred, err := p.Credential(ctx, req)
		if err != nil {
			log.Debug("skipping credential provider", "provider", providerName(p), "err", err)
			continue
		}

		if cred == nil || tried[*cred] {
			continue
		}
		tried[*cred] = true

		log.Debug("using credentials", "provider", providerName(p))
		if err := fn(cred); !isRejected(err) {
			return err
		} else if _, final := p.(Env); final {
			return err
		} else {
			log.Warn("credentials rejected", "provider", providerName(p), "err", err)
			rejected = err
		}
	}

	if rejected != nil {
		return rejected
	}

	return fn(nil)
}

// Try calls fn with each of the credentials for u in turn, as Chain.Try does
// with Default, but credentials in u's userinfo are used alone. Either way, Try
// removes u's userinfo so that it isn't sent alongside the credentials.
//
// schemes are the names that u's scheme goes by, most specific first, for
// environment variables. They default to u's scheme.
func Try(ctx context.Context, u *url.URL, location Location, schemes []string, fn func(*Credential) error) error {
	user := u.User
	u.User = nil

	if user != nil {
		if password, ok := user.Password(); ok {
			return fn(&Credential{Username: user.Username(), Password: password})
		}
	}

	if len(schemes) == 0 {
		schemes = []string{u.Scheme}
	}

	return Default.Try(ctx, &Request{
		URL:      u,
		Location: location,
		Schemes:  schemes,
	}, fn)
}

// TryReferenced is Try for a URL that barge didn't get from the user but
// found somewhere, e.g. a chart URL in a repository's index. Only credentials
// scoped to u's host are used: not u's userinfo, nor environment variables,
// which apply to every host of their scheme and so would be sent wherever an
// index points. Nor does u's query select a cloud provider, lest an index have
// barge exchange a cloud identity's token with a host of its choosing.
// TryReferenced removes u's userinfo regardless.
func TryReferenced(ctx context.Context, u *url.URL, location Location, fn func(*Credential) error) error {
	u.User = nil

	hostScoped := Chain{}
	for _, p := range Default {
		if _, ok := p.(Env); !ok {
			hostScoped = append(hostScoped, p)
		}
	}

	withoutQuery := *u
	withoutQuery.RawQuery = ""

	return hostScoped.Try(ctx, &Request{
		URL:      &withoutQuery,
		Location: location,
		Schemes:  []string{u.Scheme},
	}, fn)
}

// hostPort returns u's host with its port, defaulting the port from u's
// scheme. A scheme ending in "http", e.g. "cm+http", defaults to 80 and every
// other scheme to 443, as barge's schemes default to HTTPS.
func hostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "443"
		if strings.HasSuffix(u.Scheme, "http") {
			port = "80"
		}
	}

	return strings.ToLower(u.Hostname()) + ":" + port
}
