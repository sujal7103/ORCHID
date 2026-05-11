package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateIPRanges contains CIDR ranges for private/internal networks
var privateIPRanges = []string{
	"127.0.0.0/8",    // IPv4 loopback
	"10.0.0.0/8",     // RFC1918 private
	"172.16.0.0/12",  // RFC1918 private
	"192.168.0.0/16", // RFC1918 private
	"169.254.0.0/16", // Link-local
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // IPv6 unique local
	"fe80::/10",      // IPv6 link-local
	"0.0.0.0/8",      // "This" network
}

// blockedHostnames contains hostnames that should never be accessed
var blockedHostnames = []string{
	"localhost",
	"localhost.localdomain",
	"ip6-localhost",
	"ip6-loopback",
	"metadata.google.internal",     // GCP metadata
	"169.254.169.254",              // AWS/GCP/Azure metadata endpoint
	"metadata.google.internal.",    // GCP metadata with trailing dot
	"kubernetes.default.svc",       // Kubernetes
	"kubernetes.default",           // Kubernetes
}

var parsedCIDRs []*net.IPNet

func init() {
	// Pre-parse CIDR ranges for efficiency
	for _, cidr := range privateIPRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedCIDRs = append(parsedCIDRs, network)
		}
	}
}

// IsPrivateIP checks if an IP address is in a private/internal range
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true // Treat nil as blocked for safety
	}

	for _, network := range parsedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// IsBlockedHostname checks if a hostname is in the blocklist
func IsBlockedHostname(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))

	for _, blocked := range blockedHostnames {
		if hostname == blocked {
			return true
		}
		// Also check if it ends with the blocked hostname (subdomain matching)
		if strings.HasSuffix(hostname, "."+blocked) {
			return true
		}
	}
	return false
}

// ValidateURLForSSRF validates a URL to prevent SSRF attacks
// Returns an error if the URL points to a private/internal resource
func ValidateURLForSSRF(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http/https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only http and https schemes are allowed")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Check against blocked hostnames
	if IsBlockedHostname(hostname) {
		return fmt.Errorf("access to internal hostname '%s' is not allowed", hostname)
	}

	// Try to parse as IP address first
	ip := net.ParseIP(hostname)
	if ip != nil {
		if IsPrivateIP(ip) {
			return fmt.Errorf("access to private IP address '%s' is not allowed", hostname)
		}
		return nil
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// DNS resolution failed - allow the request to proceed
		// The actual HTTP request will fail if the host is unreachable
		return nil
	}

	// Check all resolved IPs
	for _, resolvedIP := range ips {
		if IsPrivateIP(resolvedIP) {
			return fmt.Errorf("hostname '%s' resolves to private IP address '%s'", hostname, resolvedIP.String())
		}
	}

	return nil
}

// ValidateURLForSSRFQuick performs a quick validation without DNS resolution
// Use this when DNS resolution overhead is unacceptable
func ValidateURLForSSRFQuick(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http/https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only http and https schemes are allowed")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Check against blocked hostnames
	if IsBlockedHostname(hostname) {
		return fmt.Errorf("access to internal hostname '%s' is not allowed", hostname)
	}

	// Check if hostname is an IP address
	ip := net.ParseIP(hostname)
	if ip != nil && IsPrivateIP(ip) {
		return fmt.Errorf("access to private IP address '%s' is not allowed", hostname)
	}

	return nil
}
