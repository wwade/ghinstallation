package ghinstallation

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

// AppsTransport provides a http.RoundTripper by wrapping an existing
// http.RoundTripper and provides GitHub Apps authentication as a
// GitHub App.
//
// Client can also be overwritten, and is useful to change to one which
// provides retry logic if you do experience retryable errors.
//
// See https://developer.github.com/apps/building-integrations/setting-up-and-registering-github-apps/about-authentication-options-for-github-apps/
type AppsTransport struct {
	BaseURL string            // BaseURL is the scheme and host for GitHub API, defaults to https://api.github.com
	Client  Client            // Client to use to refresh tokens, defaults to http.Client with provided transport
	Logger  interface{}       // Logger used to print debugging information
	tr      http.RoundTripper // tr is the underlying roundtripper being wrapped
	key     *rsa.PrivateKey   // key is the GitHub App's private key
	appID   int64             // appID is the GitHub App's ID
}

// NewAppsTransportKeyFromFile returns a AppsTransport using a private key from file.
func NewAppsTransportKeyFromFile(tr http.RoundTripper, appID int64, privateKeyFile string) (*AppsTransport, error) {
	privateKey, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read private key: %s", err)
	}
	return NewAppsTransport(tr, appID, privateKey)
}

// NewAppsTransport returns a AppsTransport using private key. The key is parsed
// and if any errors occur the error is non-nil.
//
// The provided tr http.RoundTripper should be shared between multiple
// installations to ensure reuse of underlying TCP connections.
//
// The returned Transport's RoundTrip method is safe to be used concurrently.
func NewAppsTransport(tr http.RoundTripper, appID int64, privateKey []byte) (*AppsTransport, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return nil, fmt.Errorf("could not parse private key: %s", err)
	}
	return NewAppsTransportFromPrivateKey(tr, appID, key), nil
}

// NewAppsTransportFromPrivateKey returns an AppsTransport using a crypto/rsa.(*PrivateKey).
func NewAppsTransportFromPrivateKey(tr http.RoundTripper, appID int64, key *rsa.PrivateKey) *AppsTransport {
	return &AppsTransport{
		BaseURL: apiBaseURL,
		Client:  &http.Client{Transport: tr},
		tr:      tr,
		key:     key,
		appID:   appID,
	}
}

func (t *AppsTransport) infow(msg string, keysAndValues ...interface{}) {
	switch l := t.Logger.(type) {
	case LeveledLogger:
		l.Infow(msg, keysAndValues...)
	}
}

func (t *AppsTransport) debugw(msg string, keysAndValues ...interface{}) {
	switch l := t.Logger.(type) {
	case LeveledLogger:
		l.Debugw(msg, keysAndValues...)
	}
}

// RoundTrip implements http.RoundTripper interface.
func (t *AppsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// GitHub rejects expiry and issue timestamps that are not an integer,
	// while the jwt-go library serializes to fractional timestamps.
	// Truncate them before passing to jwt-go.
	iss := time.Now().Add(-30 * time.Second).Truncate(time.Second)
	exp := iss.Add(2 * time.Minute)
	claims := &jwt.StandardClaims{
		IssuedAt:  iss.Unix(),
		ExpiresAt: exp.Unix(),
		Issuer:    strconv.FormatInt(t.appID, 10),
	}
	t.debugw("creating JWT",
		"claims", claims,
		"req.Method", req.Method,
		"req.URL", req.URL.String(),
	)
	bearer := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	ss, err := bearer.SignedString(t.key)
	if err != nil {
		return nil, fmt.Errorf("could not sign jwt: %s", err)
	}

	bearerHeader := "Bearer " + ss

	t.debugw("Setting headers",
		"Authorization", bearerHeader,
		"Accept", acceptHeader,
		"req.Method", req.Method,
		"req.URL", req.URL.String(),
	)
	req.Header.Set("Authorization", "Bearer "+ss)
	req.Header.Add("Accept", acceptHeader)

	resp, err := t.tr.RoundTrip(req)
	t.debugw("RoundTrip response",
		append(
			respKVs(resp),
			"roundtrip", fmt.Sprintf("%T %#v", t.tr, t.tr),
			"err", err,
		)...,
	)
	return resp, err
}
