# VCR Testing Documentation

This document describes how to use the VCR (Video Cassette Recorder) functionality in gcal-tui for testing and development.

## Overview

The VCR functionality allows you to record and replay Google Calendar API interactions, enabling:
- **E2E testing** without requiring actual Google Calendar API credentials
- **Reproducible testing** in CI/CD environments
- **Offline development** and testing
- **API response caching** for faster development cycles

## Command Line Usage

### Recording Mode
To record API interactions, use the `-test-mode` and `-cassette-file` flags:

```bash
./gcal-tui -test-mode -cassette-file=my_recording.yaml
```

**Requirements for recording:**
- Valid Google Calendar API credentials
- Internet connection to Google Calendar API
- Write permissions to the cassette file location

**What happens during recording:**
1. The application makes real API calls to Google Calendar
2. All HTTP requests and responses are recorded to the cassette file
3. Sensitive headers (Authorization, Cookie, etc.) are automatically filtered out
4. The application functions normally from a user perspective

### Replay Mode
To replay recorded interactions, use the same flags with an existing cassette file:

```bash
./gcal-tui -test-mode -cassette-file=my_recording.yaml
```

**Requirements for replaying:**
- An existing cassette file with recorded interactions
- No internet connection required
- No Google Calendar API credentials required

**What happens during replay:**
1. The application loads the cassette file instead of making real API calls
2. Recorded responses are returned for matching requests
3. The application functions as if it were connected to the real API
4. All interactions are deterministic and repeatable

## Security Features

### Automatic Header Filtering
The VCR implementation automatically removes sensitive headers to prevent credentials from being stored in cassette files:

**Request headers filtered:**
- `Authorization`
- `Cookie`
- `X-Goog-Iap-Jwt-Assertion`

**Response headers filtered:**
- `Set-Cookie`
- `X-Goog-Iap-Jwt-Assertion`

### Safe Storage
- Cassette files contain only non-sensitive HTTP interaction data
- OAuth2 tokens and credentials are never stored in cassettes
- Cassette files can be safely committed to version control

## Integration Testing

### Running Tests
```bash
# Run all tests including VCR tests
go test -v

# Run only VCR-related tests
go test -v -run TestVCR
```

### Test Environment Variables
- `GOOGLE_CREDENTIALS_FILE`: Path to Google Calendar API credentials (for recording tests)

### Example Test Scenarios

#### Recording Test
```go
func TestVCRRecording(t *testing.T) {
    client, err := createOAuth2Client(oauth2Config, true, "test.yaml")
    // Test that VCR recording is properly configured
}
```

#### Replay Test
```go
func TestVCRReplaying(t *testing.T) {
    // Create cassette file with dummy data
    client, err := createOAuth2Client(oauth2Config, true, "existing_cassette.yaml")
    // Test that VCR replay works without credentials
}
```

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Test with VCR
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - uses: actions/setup-go@v2
      with:
        go-version: 1.18
    
    - name: Run VCR Tests
      run: go test -v ./...
    
    - name: Test with existing cassette
      run: |
        ./gcal-tui -test-mode -cassette-file=fixtures/sample_events.yaml -format="{{.Summary}}"
```

### Benefits for CI/CD
- **No API credentials required** in CI environment
- **Deterministic test results** - same responses every time
- **Faster test execution** - no network calls to external APIs
- **Offline testing** - works without internet connection

## Development Workflow

### 1. Initial Recording
```bash
# Record a typical user session
./gcal-tui -test-mode -cassette-file=fixtures/daily_events.yaml -date=2025-01-15
```

### 2. Test Development
```bash
# Use recorded data for testing
./gcal-tui -test-mode -cassette-file=fixtures/daily_events.yaml -date=2025-01-15
```

### 3. Automated Testing
```bash
# Run unit tests with VCR
go test -v

# Test specific scenarios with recorded data
./gcal-tui -test-mode -cassette-file=fixtures/conflict_events.yaml
```

## Cassette File Format

Cassette files are stored in YAML format and contain:
- HTTP request details (URL, method, headers, body)
- HTTP response details (status, headers, body)
- Timestamp of recording
- Interaction metadata

Example cassette structure:
```yaml
---
http_interactions:
- request:
    body: ""
    form: {}
    headers:
      User-Agent:
      - gcal-tui/1.0
    url: https://www.googleapis.com/calendar/v3/calendars/primary/events
    method: GET
  response:
    body: '{"items": [...]}'
    headers:
      Content-Type:
      - application/json
    status:
      code: 200
      message: OK
recorded_at: 2025-01-01T00:00:00Z
```

## Best Practices

### Recording
1. **Use descriptive cassette names** that indicate the test scenario
2. **Keep cassettes focused** - record only what's needed for each test
3. **Update cassettes regularly** to reflect API changes
4. **Verify security** - ensure no sensitive data is recorded

### Replaying
1. **Version control cassettes** - commit them to your repository
2. **Document test scenarios** - explain what each cassette represents
3. **Validate responses** - ensure recorded data matches expected format
4. **Handle missing interactions** - gracefully handle cases where recorded data doesn't match requests

### Maintenance
1. **Regular updates** - re-record cassettes when API changes
2. **Cleanup** - remove unused or outdated cassettes
3. **Testing** - verify both recording and replay modes work correctly
4. **Documentation** - keep this documentation updated with new scenarios

## Troubleshooting

### Common Issues

**Issue: "cassette file not found"**
- Solution: Ensure the cassette file path is correct and file exists for replay mode

**Issue: "VCR recorder initialization failed"**
- Solution: Check file permissions and ensure the directory exists

**Issue: "API calls not being recorded"**
- Solution: Verify that test mode is enabled and cassette file path is writable

**Issue: "Replayed responses don't match"**
- Solution: Ensure the request parameters match those in the recorded cassette

### Debug Mode
To debug VCR interactions, examine the cassette file contents and verify that:
1. Requests match the expected format
2. Responses contain the expected data
3. Headers are properly filtered
4. Timestamps are reasonable

## Support

For issues with VCR functionality:
1. Check this documentation for common solutions
2. Verify your cassette files are properly formatted
3. Test with both recording and replay modes
4. File an issue if problems persist