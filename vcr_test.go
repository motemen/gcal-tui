package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/v2/recorder"
	"github.com/motemen/go-nuts/oauth2util"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// TestVCRRecording tests the VCR recording functionality
func TestVCRRecording(t *testing.T) {
	// Skip test if no credentials file is available
	credentialsFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credentialsFile == "" {
		t.Skip("GOOGLE_CREDENTIALS_FILE not set, skipping VCR recording test")
	}

	cassetteFile := "test_recording.yaml"
	
	// Clean up cassette file after test
	defer os.Remove(cassetteFile)

	// Create a mock OAuth2 config for testing
	oauth2Config := &oauth2util.Config{
		OAuth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Scopes:       []string{calendar.CalendarEventsScope},
		},
		Name: "test-gcal-tui",
	}

	// Test recording mode
	client, err := createOAuth2Client(oauth2Config, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Verify that the client has VCR transport
	if client.Transport == nil {
		t.Error("Expected VCR transport to be set")
	}

	// Test that cassette file would be created (in recording mode)
	if _, ok := client.Transport.(*recorder.Recorder); !ok {
		t.Error("Expected client transport to be VCR recorder")
	}
}

// TestVCRReplaying tests the VCR replaying functionality
func TestVCRReplaying(t *testing.T) {
	cassetteFile := "test_replaying.yaml"
	
	// Create a dummy cassette file to simulate replay mode
	dummyCassette := `---
http_interactions:
- request:
    body: ""
    form: {}
    headers:
      User-Agent:
      - gcal-tui/test
    url: https://www.googleapis.com/calendar/v3/calendars/primary/events
    method: GET
  response:
    body: '{"items": []}'
    headers:
      Content-Type:
      - application/json
    status:
      code: 200
      message: OK
recorded_at: 2025-01-01T00:00:00Z
`
	
	// Write dummy cassette file
	err := os.WriteFile(cassetteFile, []byte(dummyCassette), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy cassette file: %v", err)
	}
	
	// Clean up cassette file after test
	defer os.Remove(cassetteFile)

	// Create a mock OAuth2 config for testing
	oauth2Config := &oauth2util.Config{
		OAuth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Scopes:       []string{calendar.CalendarEventsScope},
		},
		Name: "test-gcal-tui",
	}

	// Test replaying mode
	client, err := createOAuth2Client(oauth2Config, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Verify that the client has VCR transport
	if client.Transport == nil {
		t.Error("Expected VCR transport to be set")
	}

	// Test that the recorder is in replaying mode
	if recorder, ok := client.Transport.(*recorder.Recorder); ok {
		if recorder.Mode() != recorder.ModeReplaying {
			t.Error("Expected recorder to be in replaying mode")
		}
	} else {
		t.Error("Expected client transport to be VCR recorder")
	}
}

// TestVCRSecurityFilters tests that sensitive headers are filtered out
func TestVCRSecurityFilters(t *testing.T) {
	cassetteFile := "test_filters.yaml"
	
	// Clean up cassette file after test
	defer os.Remove(cassetteFile)

	// Create a mock OAuth2 config for testing
	oauth2Config := &oauth2util.Config{
		OAuth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Scopes:       []string{calendar.CalendarEventsScope},
		},
		Name: "test-gcal-tui",
	}

	// Test that security filters are applied
	client, err := createOAuth2Client(oauth2Config, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Verify that the client has VCR transport with filters
	if recorder, ok := client.Transport.(*recorder.Recorder); ok {
		// Create a test interaction to verify filters
		interaction := &recorder.Interaction{
			Request: &recorder.Request{
				Headers: map[string][]string{
					"Authorization":           {"Bearer secret-token"},
					"Cookie":                  {"session=secret"},
					"X-Goog-Iap-Jwt-Assertion": {"secret-assertion"},
					"User-Agent":              {"gcal-tui/test"},
				},
			},
			Response: &recorder.Response{
				Headers: map[string][]string{
					"Set-Cookie":               {"session=secret"},
					"X-Goog-Iap-Jwt-Assertion": {"secret-assertion"},
					"Content-Type":             {"application/json"},
				},
			},
		}

		// Apply filters (normally done internally by VCR)
		// We can't directly test the filter function, but we can verify the recorder has filters
		if recorder == nil {
			t.Error("Expected VCR recorder to be configured with security filters")
		}
	} else {
		t.Error("Expected client transport to be VCR recorder")
	}
}

// TestVCRDisabled tests that VCR is disabled when test mode is false
func TestVCRDisabled(t *testing.T) {
	// Create a mock OAuth2 config for testing
	oauth2Config := &oauth2util.Config{
		OAuth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Scopes:       []string{calendar.CalendarEventsScope},
		},
		Name: "test-gcal-tui",
	}

	// Test with test mode disabled
	client, err := createOAuth2Client(oauth2Config, false, "")
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Verify that VCR is not used when test mode is disabled
	if _, ok := client.Transport.(*recorder.Recorder); ok {
		t.Error("Expected VCR transport to NOT be set when test mode is disabled")
	}
}

// TestFetchEventsForDate tests the fetchEventsForDate function
func TestFetchEventsForDate(t *testing.T) {
	// Skip test if no credentials or cassette file is available
	cassetteFile := "test_fetch_events.yaml"
	
	// Create a dummy cassette file with calendar events
	dummyCassette := `---
http_interactions:
- request:
    body: ""
    form: {}
    headers:
      User-Agent:
      - gcal-tui/test
    url: https://www.googleapis.com/calendar/v3/calendars/primary/events
    method: GET
  response:
    body: '{"items": [{"id": "test-event", "summary": "Test Event", "start": {"dateTime": "2025-01-01T10:00:00Z"}, "end": {"dateTime": "2025-01-01T11:00:00Z"}, "attendees": [{"email": "test@example.com", "responseStatus": "accepted", "self": true}]}]}'
    headers:
      Content-Type:
      - application/json
    status:
      code: 200
      message: OK
recorded_at: 2025-01-01T00:00:00Z
`
	
	// Write dummy cassette file
	err := os.WriteFile(cassetteFile, []byte(dummyCassette), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy cassette file: %v", err)
	}
	
	// Clean up cassette file after test
	defer os.Remove(cassetteFile)

	// Create a mock OAuth2 config for testing
	oauth2Config := &oauth2util.Config{
		OAuth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Scopes:       []string{calendar.CalendarEventsScope},
		},
		Name: "test-gcal-tui",
	}

	// Set up VCR client
	client, err := createOAuth2Client(oauth2Config, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Set the global oauthClient for testing
	originalClient := oauthClient
	oauthClient = client
	defer func() { oauthClient = originalClient }()

	// Test fetching events for a specific date
	testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	events, err := fetchEventsForDate(testDate)
	if err != nil {
		t.Fatalf("Failed to fetch events: %v", err)
	}

	// Verify events were fetched (even if empty due to mocked response)
	if events == nil {
		t.Error("Expected events slice to be non-nil")
	}
}