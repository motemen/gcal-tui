# VCR Testing Documentation

This document explains how to use the VCR (Video Cassette Recorder) functionality in gcal-tui for testing without making actual API calls to Google Calendar.

## Overview

The VCR functionality allows you to:
1. **Record** actual API requests and responses to a "cassette" file
2. **Replay** those recorded interactions for testing without hitting the real API

## Command Line Flags

Two new flags have been added:

- `-test-mode`: Enable test mode with VCR recording/replay
- `-cassette-file`: Path to the VCR cassette file (required when test-mode is enabled)

## Usage Examples

### Recording API Calls

To record actual API calls to a cassette file:

```bash
# Record API calls for today's events
./gcal-tui -test-mode -cassette-file=test_today.yaml

# Record API calls for a specific date
./gcal-tui -test-mode -cassette-file=test_date.yaml -date=2024-01-15

# Record API calls with template output
./gcal-tui -test-mode -cassette-file=test_template.yaml -format="{{.Summary}}"
```

### Replaying Recorded Calls

Once you have a cassette file, you can replay it without requiring actual API credentials:

```bash
# Replay recorded calls
./gcal-tui -test-mode -cassette-file=test_today.yaml
```

## Integration Testing

The VCR functionality is particularly useful for:

1. **CI/CD Testing**: Run tests in continuous integration without API credentials
2. **Development**: Test against consistent data sets
3. **Offline Testing**: Test functionality without internet access

### Example Test Workflow

```bash
# Step 1: Record a cassette with your actual credentials
./gcal-tui -test-mode -cassette-file=integration_test.yaml -date=2024-01-15

# Step 2: Use the cassette in your CI/CD pipeline
./gcal-tui -test-mode -cassette-file=integration_test.yaml -date=2024-01-15
```

## Cassette File Format

Cassette files are stored in YAML format and contain:
- HTTP request details (method, URL, headers)
- HTTP response details (status, headers, body)
- Sensitive information is automatically filtered out

## Security Features

The VCR implementation includes security filters that automatically remove:
- Authorization headers
- Cookie headers
- Set-Cookie headers

This ensures that sensitive authentication information is not stored in cassette files.

## Running Tests

To run the VCR tests:

```bash
# Run all tests
go test

# Run specific VCR tests
go test -run TestVCR

# Run with verbose output
go test -v -run TestVCR
```

## Limitations

- Test mode requires a cassette file to be specified
- The first run must be with actual credentials to create the cassette
- Recorded interactions are static and won't reflect real-time changes
- Time-sensitive tests may need new recordings periodically

## Best Practices

1. **Commit cassette files** to version control for consistent test results
2. **Use descriptive names** for cassette files (e.g., `events_2024_01_15.yaml`)
3. **Record multiple scenarios** (different dates, empty calendars, etc.)
4. **Update cassettes** when API responses change significantly
5. **Keep cassettes minimal** - only record what's needed for your tests