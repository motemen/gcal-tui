# VCR Recording and Replay for E2E Testing

This document describes how to use the VCR (Video Cassette Recorder) functionality for end-to-end testing of gcal-tui without requiring actual Google Calendar API credentials.

## Overview

The VCR functionality allows you to:
1. **Record** actual Google Calendar API interactions to a "cassette" file
2. **Replay** those interactions later for testing without needing API credentials

## Command Line Flags

### `-test-mode`
Enables VCR recording/replay mode. When this flag is set, the application will use VCR to either record or replay HTTP interactions.

### `-cassette-file`
Specifies the path to the VCR cassette file. This file will contain the recorded HTTP interactions.

**Required when `-test-mode` is enabled.**

## Usage

### Recording Mode

To record actual API interactions:

```bash
./gcal-tui -test-mode -cassette-file=my_test.yaml
```

**Requirements for recording:**
- Valid Google Calendar API credentials file
- Internet connection to Google Calendar API
- The cassette file should not exist (or will be overwritten)

**What happens:**
- The application makes real API calls to Google Calendar
- All HTTP requests/responses are recorded to the cassette file
- Sensitive headers (Authorization, Cookie) are automatically filtered out
- The application behaves normally

### Replay Mode

To replay recorded interactions:

```bash
./gcal-tui -test-mode -cassette-file=my_test.yaml
```

**Requirements for replay:**
- The cassette file must exist
- No internet connection or API credentials required

**What happens:**
- The application uses pre-recorded responses from the cassette file
- No actual API calls are made
- The application behaves as if it's receiving real API responses
- Perfect for CI/CD environments

## Examples

### Basic Recording and Replay

```bash
# 1. Record a session (requires credentials)
./gcal-tui -test-mode -cassette-file=today_events.yaml

# 2. Replay the session (no credentials needed)
./gcal-tui -test-mode -cassette-file=today_events.yaml
```

### Template Mode with VCR

```bash
# Record template output for a specific date
./gcal-tui -test-mode -cassette-file=template_test.yaml -format="{{.Summary}}" -date="2024-01-15"

# Replay the same template output
./gcal-tui -test-mode -cassette-file=template_test.yaml -format="{{.Summary}}" -date="2024-01-15"
```

## Security Features

VCR automatically filters out sensitive information:
- `Authorization` headers
- `Cookie` headers  
- `Set-Cookie` response headers

This ensures that recorded cassettes don't contain sensitive credentials.

## Testing

### Unit Tests

Run the VCR tests:

```bash
go test -v -run TestVCR
```

**Note:** Tests require a `test_credentials.json` file in the project root for recording mode tests.

### Integration Testing

1. Create a cassette file with recorded interactions:
   ```bash
   ./gcal-tui -test-mode -cassette-file=integration_test.yaml
   ```

2. Use the cassette file in your CI/CD pipeline:
   ```bash
   ./gcal-tui -test-mode -cassette-file=integration_test.yaml -format="{{.Summary}}"
   ```

## Cassette File Format

Cassette files are stored in YAML format and contain:
- HTTP request details (method, URL, headers, body)
- HTTP response details (status, headers, body)
- Interaction metadata

Example cassette structure:
```yaml
version: 2
interactions:
- request:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    method: GET
    url: https://www.googleapis.com/calendar/v3/calendars/primary/events
    headers:
      # Authorization header filtered out
    body: ""
  response:
    proto: HTTP/1.1
    proto_major: 1
    proto_minor: 1
    status: 200 OK
    code: 200
    headers:
      Content-Type: [application/json]
    body: |
      {
        "items": [...]
      }
```

## Troubleshooting

### Common Issues

1. **"--cassette-file is required when --test-mode is enabled"**
   - Solution: Always specify a cassette file when using test mode

2. **"oauth2Config is required for recording mode"**
   - Solution: Ensure you have valid credentials when recording

3. **"Unable to read credentials file"**
   - Solution: This is expected in replay mode when cassette file exists

4. **Empty or corrupted cassette file**
   - Solution: Delete the cassette file and re-record

### Best Practices

1. **Version Control**: Commit cassette files to version control for reproducible tests
2. **Security**: Review cassette files to ensure no sensitive data is included
3. **Maintenance**: Re-record cassettes when API responses change
4. **Organization**: Use descriptive names for cassette files (e.g., `today_events.yaml`, `weekly_summary.yaml`)

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: E2E Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - uses: actions/setup-go@v2
      with:
        go-version: 1.18
    
    - name: Build application
      run: go build -o gcal-tui
    
    - name: Run E2E tests with VCR
      run: |
        ./gcal-tui -test-mode -cassette-file=test_cassette.yaml -format="{{.Summary}}"
        ./gcal-tui -test-mode -cassette-file=test_cassette.yaml -date="+1d"
```

This allows full end-to-end testing of the compiled binary without requiring API credentials in the CI environment.