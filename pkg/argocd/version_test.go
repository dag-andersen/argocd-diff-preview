package argocd

import (
	"testing"
)

func TestExtractMajorMinorVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		expectedMajor int
		expectedMinor int
		expectError   bool
	}{
		{
			name:          "standard version with v prefix",
			version:       "v3.2.2+8d0dde1",
			expectedMajor: 3,
			expectedMinor: 2,
			expectError:   false,
		},
		{
			name:          "version without v prefix",
			version:       "3.2.2+8d0dde1",
			expectedMajor: 3,
			expectedMinor: 2,
			expectError:   false,
		},
		{
			name:          "simple version",
			version:       "v2.8.0",
			expectedMajor: 2,
			expectedMinor: 8,
			expectError:   false,
		},
		{
			name:          "major version only",
			version:       "v3",
			expectedMajor: 3,
			expectedMinor: 0,
			expectError:   false,
		},
		{
			name:          "version with double digits",
			version:       "v10.15.3",
			expectedMajor: 10,
			expectedMinor: 15,
			expectError:   false,
		},
		{
			name:          "version with build metadata",
			version:       "v2.9.1-rc1+build.123",
			expectedMajor: 2,
			expectedMinor: 9,
			expectError:   false,
		},
		{
			name:        "invalid major version",
			version:     "vX.2.1",
			expectError: true,
		},
		{
			name:        "invalid minor version",
			version:     "v3.Y.1",
			expectError: true,
		},
		{
			name:        "empty string",
			version:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, err := extractMajorMinorVersion(tt.version)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if major != tt.expectedMajor {
				t.Errorf("major version: got %d, want %d", major, tt.expectedMajor)
			}
			if minor != tt.expectedMinor {
				t.Errorf("minor version: got %d, want %d", minor, tt.expectedMinor)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{input: 5, expected: 5},
		{input: -5, expected: 5},
		{input: 0, expected: 0},
		{input: -100, expected: 100},
		{input: 100, expected: 100},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := abs(tt.input)
			if result != tt.expected {
				t.Errorf("abs(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCheckVersionDrift(t *testing.T) {
	tests := []struct {
		name             string
		clientMajor      int
		clientMinor      int
		serverMajor      int
		serverMinor      int
		expectMajorDrift bool
		expectMinorDrift bool
	}{
		{
			name:             "identical versions",
			clientMajor:      3,
			clientMinor:      2,
			serverMajor:      3,
			serverMinor:      2,
			expectMajorDrift: false,
			expectMinorDrift: false,
		},
		{
			name:             "major version differs by 1",
			clientMajor:      3,
			clientMinor:      2,
			serverMajor:      2,
			serverMinor:      2,
			expectMajorDrift: true,
			expectMinorDrift: false,
		},
		{
			name:             "minor version differs within allowed drift",
			clientMajor:      3,
			clientMinor:      5,
			serverMajor:      3,
			serverMinor:      2,
			expectMajorDrift: false,
			expectMinorDrift: false,
		},
		{
			name:             "minor version differs at max allowed drift",
			clientMajor:      3,
			clientMinor:      2 + maxMinorVersionDriftAllowed,
			serverMajor:      3,
			serverMinor:      2,
			expectMajorDrift: false,
			expectMinorDrift: false,
		},
		{
			name:             "minor version exceeds allowed drift",
			clientMajor:      3,
			clientMinor:      2 + maxMinorVersionDriftAllowed + 1,
			serverMajor:      3,
			serverMinor:      2,
			expectMajorDrift: false,
			expectMinorDrift: true,
		},
		{
			name:             "minor version exceeds allowed drift (server ahead)",
			clientMajor:      3,
			clientMinor:      2,
			serverMajor:      3,
			serverMinor:      2 + maxMinorVersionDriftAllowed + 1,
			expectMajorDrift: false,
			expectMinorDrift: true,
		},
		{
			name:             "both major and minor drift",
			clientMajor:      3,
			clientMinor:      10,
			serverMajor:      2,
			serverMinor:      2,
			expectMajorDrift: true,
			expectMinorDrift: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			majorDrift, minorDrift := checkVersionDrift(tt.clientMajor, tt.clientMinor, tt.serverMajor, tt.serverMinor)

			if majorDrift != tt.expectMajorDrift {
				t.Errorf("majorDrift: got %v, want %v", majorDrift, tt.expectMajorDrift)
			}
			if minorDrift != tt.expectMinorDrift {
				t.Errorf("minorDrift: got %v, want %v", minorDrift, tt.expectMinorDrift)
			}
		})
	}
}
