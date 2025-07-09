package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/motemen/go-nuts/oauth2util"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func TestVCRRecordingAndReplay(t *testing.T) {
	// Skip test if no credentials file is available
	credentialsFile := "test_credentials.json"
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		t.Skip("Skipping VCR test: test_credentials.json not found")
	}

	cassetteFile := "test_cassette.yaml"
	
	// Clean up cassette file before test
	os.Remove(cassetteFile)
	defer os.Remove(cassetteFile)

	// Test recording mode
	t.Run("Recording", func(t *testing.T) {
		// Load credentials
		b, err := os.ReadFile(credentialsFile)
		if err != nil {
			t.Fatalf("Failed to read credentials file: %v", err)
		}

		oauth2Config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
		if err != nil {
			t.Fatalf("Failed to parse credentials: %v", err)
		}

		// Create OAuth2 client with VCR recording
		client, err := createOAuth2Client(context.Background(), &oauth2util.Config{
			OAuth2Config: oauth2Config,
			Name:         "gcal-tui-test",
		}, true, cassetteFile)
		if err != nil {
			t.Fatalf("Failed to create OAuth2 client: %v", err)
		}

		// Make a test API call
		calendarService, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			t.Fatalf("Failed to create calendar service: %v", err)
		}

		// Test fetching events for today
		today := time.Now().Truncate(24 * time.Hour)
		_, err = calendarService.Events.List("primary").
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(today.Format(time.RFC3339)).
			TimeMax(today.Add(24 * time.Hour).Format(time.RFC3339)).
			OrderBy("startTime").
			Do()
		if err != nil {
			t.Fatalf("Failed to fetch events: %v", err)
		}

		// Verify cassette file was created
		if _, err := os.Stat(cassetteFile); os.IsNotExist(err) {
			t.Fatalf("Cassette file was not created")
		}
	})

	// Test replay mode
	t.Run("Replay", func(t *testing.T) {
		// Verify cassette file exists
		if _, err := os.Stat(cassetteFile); os.IsNotExist(err) {
			t.Fatalf("Cassette file does not exist, recording test may have failed")
		}

		// Create OAuth2 client with VCR replay (no credentials needed)
		client, err := createOAuth2Client(context.Background(), nil, true, cassetteFile)
		if err != nil {
			t.Fatalf("Failed to create OAuth2 client for replay: %v", err)
		}

		// Make the same API call - should use recorded response
		calendarService, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			t.Fatalf("Failed to create calendar service: %v", err)
		}

		// Test fetching events for today
		today := time.Now().Truncate(24 * time.Hour)
		_, err = calendarService.Events.List("primary").
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(today.Format(time.RFC3339)).
			TimeMax(today.Add(24 * time.Hour).Format(time.RFC3339)).
			OrderBy("startTime").
			Do()
		if err != nil {
			t.Fatalf("Failed to fetch events in replay mode: %v", err)
		}
	})
}

func TestVCRFlagsValidation(t *testing.T) {
	// Test that createOAuth2Client validates flags properly
	t.Run("MissingCassetteFile", func(t *testing.T) {
		_, err := createOAuth2Client(context.Background(), nil, true, "")
		if err == nil {
			t.Fatal("Expected error for empty cassette file")
		}
	})

	t.Run("NonExistentCassetteFileRecording", func(t *testing.T) {
		// This should work - recording mode with non-existent cassette file
		// But we need oauth2Config for recording
		_, err := createOAuth2Client(context.Background(), nil, true, "non-existent.yaml")
		if err == nil {
			t.Fatal("Expected error for nil oauth2Config in recording mode")
		}
	})
}

func TestFetchEventsForDate(t *testing.T) {
	// Test the fetchEventsForDate function with VCR
	cassetteFile := "test_fetch_events.yaml"
	
	// Clean up cassette file
	os.Remove(cassetteFile)
	defer os.Remove(cassetteFile)

	// Skip test if no credentials file is available
	credentialsFile := "test_credentials.json"
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		t.Skip("Skipping fetchEventsForDate test: test_credentials.json not found")
	}

	// Load credentials and create OAuth2 client
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		t.Skipf("Failed to read credentials file: %v", err)
	}

	oauth2Config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		t.Fatalf("Failed to parse credentials: %v", err)
	}

	// Record mode
	oauthClient, err = createOAuth2Client(context.Background(), &oauth2util.Config{
		OAuth2Config: oauth2Config,
		Name:         "gcal-tui-test",
	}, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client: %v", err)
	}

	// Test fetching events for today
	today := time.Now().Truncate(24 * time.Hour)
	events, err := fetchEventsForDate(today)
	if err != nil {
		t.Fatalf("Failed to fetch events: %v", err)
	}

	// Should not error even if no events
	if events == nil {
		t.Fatal("Expected non-nil events slice")
	}

	// Test replay mode
	oauthClient, err = createOAuth2Client(context.Background(), nil, true, cassetteFile)
	if err != nil {
		t.Fatalf("Failed to create OAuth2 client for replay: %v", err)
	}

	// Fetch events again using recorded data
	eventsReplay, err := fetchEventsForDate(today)
	if err != nil {
		t.Fatalf("Failed to fetch events in replay mode: %v", err)
	}

	// Should return the same number of events
	if len(events) != len(eventsReplay) {
		t.Fatalf("Expected %d events in replay, got %d", len(events), len(eventsReplay))
	}
}