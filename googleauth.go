package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const program = "tui-gcal"

var (
	cacheDir  string
	tokenFile string
)

func init() {
	d, err := os.UserCacheDir()
	if err != nil {
		panic(fmt.Sprintf("os.UserCacheDir: %v", err))
	}
	cacheDir = filepath.Join(d, program)
	tokenFile = filepath.Join(cacheDir, "token.json")
}

func getHTTPClient(config *oauth2.Config) (*http.Client, error) {
	token, err := restoreToken()
	if err != nil {
		token, err = doAuthorization(config)
		if err != nil {
			return nil, err
		}

		err = storeToken(token)
		if err != nil {
			return nil, err
		}
	}

	return config.Client(context.Background(), token), nil
}

func startCodeReceiver() (<-chan string, string, func()) {
	ch := make(chan string)

	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/favicon.ico" {
				http.Error(w, "Not Found", 404)
				return
			}

			if code := r.FormValue("code"); code != "" {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprintln(w, "Authorized.")
				ch <- code
				return
			}
		}))

	return ch, s.URL, func() { s.Close() }
}

func doAuthorization(config *oauth2.Config) (*oauth2.Token, error) {
	ch, url, cancel := startCodeReceiver()
	defer cancel()

	config.RedirectURL = url

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visit below to authorize %s:\n%s\n", program, authURL)

	authCode := <-ch

	return config.Exchange(context.TODO(), authCode)
}

func restoreToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	err = json.NewDecoder(f).Decode(&token)
	return &token, err
}

func storeToken(token *oauth2.Token) error {
	err := os.MkdirAll(filepath.Dir(tokenFile), 0o777)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

func getGoogleOAuthClient(credentialsFile string, scopes []string) (*http.Client, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Error in google.ConfigFromJSON: %v", err)
	}

	return getHTTPClient(config)
}
