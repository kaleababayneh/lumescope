package background

import "testing"

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
