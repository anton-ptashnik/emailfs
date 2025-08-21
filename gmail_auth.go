package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GmailAuthorizer struct {
	c             *imapclient.Client
	tokenFilepath string
}

func (self *GmailAuthorizer) Login() (EmailInterface, error) {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://127.0.0.1:44444",
		Scopes:       []string{"https://mail.google.com/"},
		Endpoint:     google.Endpoint,
	}

	// Try to load existing token, fall back to interactive auth
	token, err := loadToken(self.tokenFilepath)
	if err != nil {
		token, err = interactiveAuth(conf)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		saveToken(self.tokenFilepath, token)
	}

	// Get fresh token (handles refresh automatically)
	tokenSrc := conf.TokenSource(context.Background(), token)
	freshToken, err := tokenSrc.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	email := os.Getenv("EMAIL_ADDRESS")
	xoauth2 := NewXOAuth2(email, freshToken.AccessToken)

	if err := self.c.Authenticate(xoauth2); err != nil {
		return nil, fmt.Errorf("IMAP authentication failed: %w", err)
	}

	return &GoImapEmailInterface{c: self.c}, nil
}

func (s *GmailAuthorizer) Logout() error {
	return s.c.Close()
}

func NewGAuth(tokenFilepath string) (*GmailAuthorizer, error) {
	c, err := imapclient.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return nil, err
	}
	return &GmailAuthorizer{c: c, tokenFilepath: tokenFilepath}, nil
}

func interactiveAuth(conf *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:44444")
	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	state := "state123"
	authURL := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)

	openBrowser(authURL)
	fmt.Println("If browser didn't open, visit:", authURL)

	codeCh := make(chan string, 1)
	go func() {
		http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("state") != state {
				http.Error(w, "Invalid state", http.StatusBadRequest)
				return
			}
			code := r.URL.Query().Get("code")
			io.WriteString(w, "Authorization complete. You can close this tab.")
			codeCh <- code
		}))
	}()

	code := <-codeCh
	return conf.Exchange(context.Background(), code)
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

// XOAuth2 returns a sasl.Client for Gmail OAuth2
func NewXOAuth2(username, accessToken string) sasl.Client {
	return &xoauth2Client{username, accessToken}
}

type xoauth2Client struct {
	username, accessToken string
}

func (a *xoauth2Client) Start() (mech string, ir []byte, err error) {
	return "XOAUTH2", []byte(fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.username, a.accessToken)), nil
}

func (a *xoauth2Client) Next(challenge []byte) (response []byte, err error) {
	return nil, nil
}
