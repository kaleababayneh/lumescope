package background

import "testing"

func TestExtractMimeType(t *testing.T) {
	tests := []struct {
		name     string
		decoded  map[string]any
		expected string
	}{
		// Nil and empty cases
		{
			name:     "nil decoded map returns octet-stream",
			decoded:  nil,
			expected: "application/octet-stream",
		},
		{
			name:     "empty decoded map returns octet-stream",
			decoded:  map[string]any{},
			expected: "application/octet-stream",
		},
		{
			name:     "missing file_name returns octet-stream",
			decoded:  map[string]any{"other_field": "value"},
			expected: "application/octet-stream",
		},
		{
			name:     "empty file_name returns octet-stream",
			decoded:  map[string]any{"file_name": ""},
			expected: "application/octet-stream",
		},
		{
			name:     "file_name without extension returns octet-stream",
			decoded:  map[string]any{"file_name": "myfile"},
			expected: "application/octet-stream",
		},

		// Valid MIME type detection
		{
			name:     "jpeg file",
			decoded:  map[string]any{"file_name": "photo.jpg"},
			expected: "image/jpeg",
		},
		{
			name:     "jpeg file uppercase",
			decoded:  map[string]any{"file_name": "photo.JPG"},
			expected: "image/jpeg",
		},
		{
			name:     "png file",
			decoded:  map[string]any{"file_name": "image.png"},
			expected: "image/png",
		},
		{
			name:     "pdf file",
			decoded:  map[string]any{"file_name": "document.pdf"},
			expected: "application/pdf",
		},
		{
			name:     "text file - charset stripped",
			decoded:  map[string]any{"file_name": "readme.txt"},
			expected: "text/plain",
		},
		{
			name:     "html file - charset stripped",
			decoded:  map[string]any{"file_name": "index.html"},
			expected: "text/html",
		},
		{
			name:     "json file",
			decoded:  map[string]any{"file_name": "data.json"},
			expected: "application/json",
		},
		{
			name:     "zip file",
			decoded:  map[string]any{"file_name": "archive.zip"},
			expected: "application/zip",
		},
		{
			name:     "mp4 file",
			decoded:  map[string]any{"file_name": "video.mp4"},
			expected: "video/mp4",
		},
		{
			name:     "gif file",
			decoded:  map[string]any{"file_name": "animation.gif"},
			expected: "image/gif",
		},

		// Unknown extensions
		{
			name:     "unknown extension returns octet-stream",
			decoded:  map[string]any{"file_name": "file.xyz123"},
			expected: "application/octet-stream",
		},
		{
			name:     "unknown extension .custom",
			decoded:  map[string]any{"file_name": "data.custom"},
			expected: "application/octet-stream",
		},

		// Edge cases
		{
			name:     "multiple dots in filename",
			decoded:  map[string]any{"file_name": "my.file.photo.jpg"},
			expected: "image/jpeg",
		},
		{
			name:     "hidden file with extension",
			decoded:  map[string]any{"file_name": ".hidden.txt"},
			expected: "text/plain",
		},
		{
			name:     "file_name is not a string",
			decoded:  map[string]any{"file_name": 12345},
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMimeType(tt.decoded)
			if got != tt.expected {
				t.Errorf("extractMimeType(%v) = %q, want %q", tt.decoded, got, tt.expected)
			}
		})
	}
}

func TestIsValidHost(t *testing.T) {
	tests := []struct {
		host  string
		valid bool
		desc  string
	}{
		// Valid IP addresses
		{"192.168.1.1", true, "valid IPv4"},
		{"10.0.0.1", true, "valid IPv4 private"},
		{"::1", true, "valid IPv6 loopback"},
		{"2001:db8::1", true, "valid IPv6"},

		// Valid hostnames/FQDN
		{"example.com", true, "valid FQDN"},
		{"sub.example.com", true, "valid subdomain"},
		{"my-server.example.com", true, "valid FQDN with hyphen"},
		{"server1.example.com", true, "valid FQDN with number"},
		{"a.b.c.d.example.com", true, "valid multi-level subdomain"},

		// Invalid hosts (including single-label hostnames without dots)
		{"SUNUCUIP", false, "single-label placeholder - rejected"},
		{"localhost", false, "single-label hostname - rejected for production"},
		{"server1", false, "single-label hostname - rejected for production"},
		{"", false, "empty string"},
		{".", false, "just dot"},
		{".example.com", false, "starts with dot"},
		{"example.com.", false, "ends with dot"},
		{"example..com", false, "consecutive dots"},
		{"-example.com", false, "starts with hyphen"},
		{"example.com-", false, "ends with hyphen"},
		{"exam ple.com", false, "contains space"},
		{"example$.com", false, "invalid character"},
		{"123", false, "only numbers, no letters"},
		{"12.34", false, "only numbers with dot, no letters"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := isValidHost(tt.host)
			if got != tt.valid {
				t.Errorf("isValidHost(%q) = %v, want %v (%s)", tt.host, got, tt.valid, tt.desc)
			}
		})
	}
}
