package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

const (
	// CallbackPort is the port for the OAuth callback server
	CallbackPort = 8089
	// AuthTimeout is how long to wait for the user to complete auth
	AuthTimeout = 5 * time.Minute
)

// Authenticate runs the OAuth flow with a local callback server
func Authenticate(ctx context.Context, cfg *oauth2.Config) (*AuthResult, error) {
	// Generate state for CSRF protection
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	// Channel to receive the auth code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create server mux (don't use DefaultServeMux)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch - possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errChan <- fmt.Errorf("auth error: %s", errMsg)
			http.Error(w, "Authentication failed", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			http.Error(w, "No authorization code", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body style="font-family: system-ui; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0;">
<div style="text-align: center;">
<h1 style="color: #10B981;">Success!</h1>
<p>You can close this window and return to the terminal.</p>
</div>
</body>
</html>`)
		codeChan <- code
	})

	// Start local server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", CallbackPort))
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}

	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Generate auth URL and prompt user
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Println()
	fmt.Println("To authenticate with Strava, open this URL in your browser:")
	fmt.Println()
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Println("Waiting for authentication...")

	// Wait for callback with timeout
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		shutdownServer(server)
		return nil, err
	case <-time.After(AuthTimeout):
		shutdownServer(server)
		return nil, fmt.Errorf("authentication timeout after %v", AuthTimeout)
	case <-ctx.Done():
		shutdownServer(server)
		return nil, ctx.Err()
	}

	shutdownServer(server)

	// Exchange code for token
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}

	// Extract athlete ID from token extras
	athleteID := ExtractAthleteID(token)

	return &AuthResult{
		Token:     token,
		AthleteID: athleteID,
	}, nil
}

// generateState creates a random state string for CSRF protection
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// shutdownServer gracefully shuts down the HTTP server
func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
