package googlecal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const authTimeout = 5 * time.Minute

var calendarScopes = []string{
	"https://www.googleapis.com/auth/calendar.events",
	"https://www.googleapis.com/auth/calendar.freebusy",
}

// Authenticate runs the OAuth desktop flow and writes the resulting token.
func Authenticate(ctx context.Context, credentialsPath, tokenPath string, stdout io.Writer) error {
	oauthConfig, err := readOAuthConfig(credentialsPath)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	state, err := randomState()
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	server := &http.Server{
		Handler: authHandler(state, codeCh),
	}
	defer server.Shutdown(context.Background())

	go func() {
		_ = server.Serve(listener)
	}()

	oauthConfig.RedirectURL = "http://" + listener.Addr().String() + "/callback"
	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Fprintln(stdout, "Open this URL in your browser:")
	fmt.Fprintln(stdout, url)

	waitCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	var code string
	select {
	case <-waitCtx.Done():
		return waitCtx.Err()
	case code = <-codeCh:
	}

	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange auth code: %w", err)
	}
	return saveToken(tokenPath, token)
}

// NewService creates a Calendar service from local OAuth files.
func NewService(ctx context.Context, credentialsPath, tokenPath, calendarID string) (*Service, error) {
	oauthConfig, err := readOAuthConfig(credentialsPath)
	if err != nil {
		return nil, err
	}
	token, err := readToken(tokenPath)
	if err != nil {
		return nil, err
	}

	client := oauthConfig.Client(ctx, token)
	return &Service{calendarID: calendarID, client: client}, nil
}

func readOAuthConfig(path string) (*oauth2.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials %s: %w", path, err)
	}
	cfg, err := google.ConfigFromJSON(data, calendarScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials %s: %w", path, err)
	}
	return cfg, nil
}

func saveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

func readToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("parse token %s: %w", path, err)
	}
	return &token, nil
}

func authHandler(state string, codeCh chan<- string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		codeCh <- code
		fmt.Fprintln(w, "Authentication complete. You can close this tab.")
	})
	return mux
}

func randomState() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}
