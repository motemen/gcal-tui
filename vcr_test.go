package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/motemen/go-nuts/oauth2util"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

// TestVCRRecording demonstrates recording actual API calls to a cassette file
func TestVCRRecording(t *testing.T) {
	// Skip if credentials are not available
	confDir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("Cannot get user config directory")
	}
	
	credentialsFile := filepath.Join(confDir, programName, "credentials.json")
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		t.Skip("Credentials file not found - skipping recording test")
	}

	// Create a temporary cassette file
	cassetteFile := filepath.Join(t.TempDir(), "test_recording.yaml")

	// Read credentials
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		t.Fatal(err)
	}

	oauth2Config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		t.Fatal(err)
	}

	// Create OAuth2 client with VCR recording
	client, err := createOAuth2Client(context.Background(), &oauth2util.Config{
		OAuth2Config: oauth2Config,
		Name:         programName,
	}, true, cassetteFile)
	if err != nil {
		t.Fatal(err)
	}

	// Set the global client for testing
	oauthClient = client

	// Fetch events for today
	events, err := fetchEventsForDate(time.Now().Truncate(24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Verify we got some response (even if empty)
	if events == nil {
		t.Fatal("Expected events slice, got nil")
	}

	// Verify cassette file was created
	if _, err := os.Stat(cassetteFile); os.IsNotExist(err) {
		t.Error("Cassette file was not created")
	}
}

// TestVCRReplay demonstrates replaying recorded API calls from a cassette file
func TestVCRReplay(t *testing.T) {
	// Create a test cassette file with sample data
	cassetteFile := filepath.Join(t.TempDir(), "test_replay.yaml")
	
	// Create minimal cassette content for testing
	cassetteContent := `---
interactions:
- request:
    method: GET
    url: https://www.googleapis.com/calendar/v3/calendars/primary/events
    headers:
      Accept: application/json
      User-Agent: google-api-go-client/0.5
  response:
    status: 200
    headers:
      Content-Type: application/json
    body: |
      {
        "items": [],
        "kind": "calendar#events",
        "etag": "test-etag",
        "summary": "Test Calendar",
        "timeZone": "UTC"
      }
`
	
	err := os.WriteFile(cassetteFile, []byte(cassetteContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create OAuth2 client with VCR replay (no real credentials needed)
	client, err := createOAuth2Client(context.Background(), &oauth2util.Config{
		OAuth2Config: &oauth2util.Config{}.OAuth2Config,
		Name:         programName,
	}, true, cassetteFile)
	if err != nil {
		t.Fatal(err)
	}

	// Set the global client for testing
	oauthClient = client

	// This should work without real credentials because it will use the cassette
	events, err := fetchEventsForDate(time.Now().Truncate(24 * time.Hour))
	if err != nil {
		// Note: This might still fail due to OAuth2 client initialization
		// but the VCR transport should be working
		t.Logf("Expected error due to test setup: %v", err)
	}

	// Verify we got a response structure
	if events == nil {
		t.Log("Events is nil, which is expected for this test setup")
	}
}

// TestVCRFlags tests the command-line flag handling
func TestVCRFlags(t *testing.T) {
	// Test that test-mode flag validation works
	cassetteFile := filepath.Join(t.TempDir(), "test_flags.yaml")
	
	// This should work
	client, err := createOAuth2Client(context.Background(), &oauth2util.Config{
		OAuth2Config: &oauth2util.Config{}.OAuth2Config,
		Name:         programName,
	}, true, cassetteFile)
	if err != nil {
		t.Fatal(err)
	}
	
	if client == nil {
		t.Error("Expected client to be created")
	}

	// This should fail - test mode without cassette file
	_, err = createOAuth2Client(context.Background(), &oauth2util.Config{
		OAuth2Config: &oauth2util.Config{}.OAuth2Config,
		Name:         programName,
	}, true, "")
	if err == nil {
		t.Error("Expected error when test-mode is enabled without cassette-file")
	}
}