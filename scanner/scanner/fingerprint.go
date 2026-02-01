package scanner

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"
)

// ServiceInfo contains fingerprinted service information
type ServiceInfo struct {
	ServiceName    string                 `json:"service_name,omitempty"`
	ServiceVersion string                 `json:"service_version,omitempty"`
	Banner         string                 `json:"banner,omitempty"`
	Fingerprint    map[string]interface{} `json:"fingerprint_data,omitempty"`
}

// Fingerprinter handles service fingerprinting
type Fingerprinter struct {
	Timeout    time.Duration
	MaxBanner  int
}

// NewFingerprinter creates a new Fingerprinter instance
func NewFingerprinter() *Fingerprinter {
	return &Fingerprinter{
		Timeout:   5 * time.Second,
		MaxBanner: 1024,
	}
}

// FingerprintHost fingerprints services on a host's open ports
func (f *Fingerprinter) FingerprintHost(ctx context.Context, ip string, ports []int) map[int]ServiceInfo {
	results := make(map[int]ServiceInfo)

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		info := f.fingerprintPort(ctx, ip, port)
		results[port] = info
	}

	return results
}

func (f *Fingerprinter) fingerprintPort(ctx context.Context, ip string, port int) ServiceInfo {
	var info ServiceInfo

	// Try protocol-specific probes based on port
	switch port {
	case 21:
		info = f.probeFTP(ip, port)
	case 22:
		info = f.probeSSH(ip, port)
	case 23:
		info = f.probeTelnet(ip, port)
	case 25, 465, 587:
		info = f.probeSMTP(ip, port)
	case 80, 8080, 8000, 8888:
		info = f.probeHTTP(ip, port, false)
	case 443, 8443:
		info = f.probeHTTP(ip, port, true)
	case 110:
		info = f.probePOP3(ip, port)
	case 143:
		info = f.probeIMAP(ip, port)
	case 3306:
		info = f.probeMySQL(ip, port)
	case 5432:
		info = f.probePostgreSQL(ip, port)
	case 6379:
		info = f.probeRedis(ip, port)
	case 27017:
		info = f.probeMongoDB(ip, port)
	default:
		// Generic banner grab
		info = f.probeGeneric(ip, port)
	}

	// If we didn't get a service name, try to guess from banner
	if info.ServiceName == "" && info.Banner != "" {
		info.ServiceName = guessServiceFromBanner(info.Banner, port)
	}

	// Fall back to port-based service name
	if info.ServiceName == "" {
		info.ServiceName = getDefaultServiceName(port)
	}

	return info
}

// probeGeneric tries to get a banner by connecting and waiting
func (f *Fingerprinter) probeGeneric(ip string, port int) ServiceInfo {
	var info ServiceInfo
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	// Wait for banner (some services send immediately)
	buf := make([]byte, f.MaxBanner)
	n, _ := conn.Read(buf)
	if n > 0 {
		info.Banner = sanitizeBanner(string(buf[:n]))
	}

	return info
}

// probeSSH connects and reads SSH banner
func (f *Fingerprinter) probeSSH(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "ssh"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	reader := bufio.NewReader(conn)
	banner, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return info
	}

	info.Banner = sanitizeBanner(banner)

	// Parse SSH version from banner like "SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1"
	if strings.HasPrefix(banner, "SSH-") {
		parts := strings.SplitN(banner, "-", 3)
		if len(parts) >= 3 {
			info.ServiceVersion = strings.TrimSpace(parts[2])
		}
	}

	return info
}

// probeHTTP sends an HTTP request and parses response
func (f *Fingerprinter) probeHTTP(ip string, port int, useTLS bool) ServiceInfo {
	var info ServiceInfo
	if useTLS {
		info.ServiceName = "https"
	} else {
		info.ServiceName = "http"
	}
	address := fmt.Sprintf("%s:%d", ip, port)

	var conn net.Conn
	var err error

	if useTLS {
		dialer := &net.Dialer{Timeout: f.Timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
			InsecureSkipVerify: true,
		})
	} else {
		conn, err = net.DialTimeout("tcp", address, f.Timeout)
	}

	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(f.Timeout))

	// Send HTTP request
	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: NetworkScanner/1.0\r\nConnection: close\r\n\r\n", ip)
	conn.Write([]byte(request))

	// Read response
	buf := make([]byte, f.MaxBanner)
	n, _ := conn.Read(buf)
	if n > 0 {
		response := string(buf[:n])
		info.Banner = sanitizeBanner(response)

		// Extract Server header
		serverRe := regexp.MustCompile(`(?i)Server:\s*([^\r\n]+)`)
		if matches := serverRe.FindStringSubmatch(response); len(matches) > 1 {
			info.ServiceVersion = strings.TrimSpace(matches[1])
		}

		// Store additional fingerprint data
		info.Fingerprint = make(map[string]interface{})

		// Extract status code
		statusRe := regexp.MustCompile(`HTTP/[\d.]+\s+(\d+)`)
		if matches := statusRe.FindStringSubmatch(response); len(matches) > 1 {
			info.Fingerprint["status_code"] = matches[1]
		}

		// Extract title if present
		titleRe := regexp.MustCompile(`(?i)<title>([^<]+)</title>`)
		if matches := titleRe.FindStringSubmatch(response); len(matches) > 1 {
			info.Fingerprint["title"] = strings.TrimSpace(matches[1])
		}
	}

	return info
}

// probeFTP connects and reads FTP banner
func (f *Fingerprinter) probeFTP(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "ftp"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	reader := bufio.NewReader(conn)
	banner, _ := reader.ReadString('\n')
	info.Banner = sanitizeBanner(banner)

	// Parse version from banner like "220 ProFTPD 1.3.5 Server"
	if strings.HasPrefix(banner, "220") {
		info.ServiceVersion = extractVersion(banner)
	}

	return info
}

// probeTelnet connects and reads telnet banner
func (f *Fingerprinter) probeTelnet(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "telnet"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	buf := make([]byte, f.MaxBanner)
	n, _ := conn.Read(buf)
	if n > 0 {
		info.Banner = sanitizeBanner(string(buf[:n]))
	}

	return info
}

// probeSMTP connects and reads SMTP banner
func (f *Fingerprinter) probeSMTP(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "smtp"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	reader := bufio.NewReader(conn)
	banner, _ := reader.ReadString('\n')
	info.Banner = sanitizeBanner(banner)

	if strings.HasPrefix(banner, "220") {
		info.ServiceVersion = extractVersion(banner)
	}

	return info
}

// probePOP3 connects and reads POP3 banner
func (f *Fingerprinter) probePOP3(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "pop3"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	reader := bufio.NewReader(conn)
	banner, _ := reader.ReadString('\n')
	info.Banner = sanitizeBanner(banner)

	return info
}

// probeIMAP connects and reads IMAP banner
func (f *Fingerprinter) probeIMAP(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "imap"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	reader := bufio.NewReader(conn)
	banner, _ := reader.ReadString('\n')
	info.Banner = sanitizeBanner(banner)

	return info
}

// probeMySQL connects and reads MySQL handshake
func (f *Fingerprinter) probeMySQL(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "mysql"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	buf := make([]byte, f.MaxBanner)
	n, _ := conn.Read(buf)
	if n > 0 {
		// MySQL packet starts with 4-byte header, then protocol version, then null-terminated version string
		if n > 5 {
			// Find version string (starts after protocol byte, ends at null)
			start := 5
			end := start
			for end < n && buf[end] != 0 {
				end++
			}
			if end > start {
				info.ServiceVersion = string(buf[start:end])
				info.Banner = fmt.Sprintf("MySQL %s", info.ServiceVersion)
			}
		}
	}

	return info
}

// probePostgreSQL connects and reads PostgreSQL response
func (f *Fingerprinter) probePostgreSQL(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "postgresql"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	// Send SSL request to trigger a response
	// PostgreSQL SSL request: length (8 bytes) + SSL magic number
	sslRequest := []byte{0, 0, 0, 8, 4, 210, 22, 47}
	conn.Write(sslRequest)

	buf := make([]byte, 1)
	n, _ := conn.Read(buf)
	if n > 0 {
		if buf[0] == 'N' {
			info.Banner = "PostgreSQL (SSL not supported)"
		} else if buf[0] == 'S' {
			info.Banner = "PostgreSQL (SSL supported)"
		}
	}

	return info
}

// probeRedis connects and sends PING command
func (f *Fingerprinter) probeRedis(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "redis"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(f.Timeout))

	// Send Redis PING command
	conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))

	buf := make([]byte, f.MaxBanner)
	n, _ := conn.Read(buf)
	if n > 0 {
		response := string(buf[:n])
		if strings.Contains(response, "PONG") {
			info.Banner = "Redis server"
		} else if strings.Contains(response, "NOAUTH") {
			info.Banner = "Redis server (authentication required)"
		}

		// Try INFO command for version
		conn.Write([]byte("*1\r\n$4\r\nINFO\r\n"))
		n, _ = conn.Read(buf)
		if n > 0 {
			infoResp := string(buf[:n])
			versionRe := regexp.MustCompile(`redis_version:(\S+)`)
			if matches := versionRe.FindStringSubmatch(infoResp); len(matches) > 1 {
				info.ServiceVersion = matches[1]
			}
		}
	}

	return info
}

// probeMongoDB connects and sends isMaster command
func (f *Fingerprinter) probeMongoDB(ip string, port int) ServiceInfo {
	var info ServiceInfo
	info.ServiceName = "mongodb"
	address := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", address, f.Timeout)
	if err != nil {
		return info
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(f.Timeout))

	// Try to read any banner (MongoDB doesn't send one, but some proxies might)
	buf := make([]byte, f.MaxBanner)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, _ := conn.Read(buf)
	if n > 0 {
		info.Banner = sanitizeBanner(string(buf[:n]))
	} else {
		info.Banner = "MongoDB"
	}

	return info
}

// sanitizeBanner cleans up a banner string
func sanitizeBanner(s string) string {
	// Trim whitespace and control characters
	s = strings.TrimSpace(s)

	// Replace non-printable characters
	var result strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 {
			result.WriteRune(r)
		} else if r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(' ')
		}
	}

	// Truncate if too long
	out := result.String()
	if len(out) > 512 {
		out = out[:512] + "..."
	}

	return strings.TrimSpace(out)
}

// extractVersion tries to extract version info from a banner
func extractVersion(banner string) string {
	// Common version patterns
	patterns := []string{
		`(\d+\.\d+(?:\.\d+)?(?:[.-]\w+)?)`,
		`v(\d+\.\d+(?:\.\d+)?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(banner); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// guessServiceFromBanner tries to identify service from banner content
func guessServiceFromBanner(banner string, port int) string {
	lower := strings.ToLower(banner)

	switch {
	case strings.Contains(lower, "ssh"):
		return "ssh"
	case strings.Contains(lower, "ftp"):
		return "ftp"
	case strings.Contains(lower, "smtp") || strings.Contains(lower, "mail"):
		return "smtp"
	case strings.Contains(lower, "http"):
		return "http"
	case strings.Contains(lower, "mysql"):
		return "mysql"
	case strings.Contains(lower, "postgresql") || strings.Contains(lower, "postgres"):
		return "postgresql"
	case strings.Contains(lower, "redis"):
		return "redis"
	case strings.Contains(lower, "mongodb") || strings.Contains(lower, "mongo"):
		return "mongodb"
	case strings.Contains(lower, "imap"):
		return "imap"
	case strings.Contains(lower, "pop"):
		return "pop3"
	case strings.Contains(lower, "telnet"):
		return "telnet"
	}

	return ""
}

// getDefaultServiceName returns a default service name based on port
func getDefaultServiceName(port int) string {
	services := map[int]string{
		21:    "ftp",
		22:    "ssh",
		23:    "telnet",
		25:    "smtp",
		53:    "dns",
		80:    "http",
		110:   "pop3",
		143:   "imap",
		443:   "https",
		445:   "smb",
		465:   "smtps",
		587:   "submission",
		993:   "imaps",
		995:   "pop3s",
		1433:  "mssql",
		1521:  "oracle",
		3306:  "mysql",
		3389:  "rdp",
		5432:  "postgresql",
		5900:  "vnc",
		6379:  "redis",
		8080:  "http-proxy",
		8443:  "https-alt",
		27017: "mongodb",
	}

	if name, ok := services[port]; ok {
		return name
	}
	return "unknown"
}
