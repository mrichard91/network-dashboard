package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ZgrabFingerprinter uses zgrab2 for enhanced service fingerprinting
type ZgrabFingerprinter struct {
	Timeout    time.Duration
	MaxBanner  int
	Fallback   *Fingerprinter // Fallback to native fingerprinting
}

// NewZgrabFingerprinter creates a new ZgrabFingerprinter
func NewZgrabFingerprinter() *ZgrabFingerprinter {
	return &ZgrabFingerprinter{
		Timeout:   10 * time.Second,
		MaxBanner: 4096,
		Fallback:  NewFingerprinter(),
	}
}

// ZgrabResult represents the top-level zgrab2 JSON output
type ZgrabResult struct {
	IP     string                   `json:"ip"`
	Domain string                   `json:"domain,omitempty"`
	Data   map[string]*ZgrabModule  `json:"data"`
}

// ZgrabModule represents a protocol module result
type ZgrabModule struct {
	Status    string          `json:"status"`
	Protocol  string          `json:"protocol"`
	Timestamp string          `json:"timestamp"`
	Result    json.RawMessage `json:"result"`
	Error     string          `json:"error,omitempty"`
}

// TLSLog contains TLS handshake information
type TLSLog struct {
	HandshakeLog *HandshakeLog `json:"handshake_log,omitempty"`
}

// HandshakeLog contains certificate and cipher info
type HandshakeLog struct {
	ServerCertificates *ServerCertificates `json:"server_certificates,omitempty"`
	ServerHello        *ServerHello        `json:"server_hello,omitempty"`
}

// ServerCertificates contains the certificate chain
type ServerCertificates struct {
	Certificate *Certificate   `json:"certificate,omitempty"`
	Chain       []*Certificate `json:"chain,omitempty"`
}

// Certificate represents an X.509 certificate
type Certificate struct {
	Raw    string          `json:"raw,omitempty"`
	Parsed *ParsedCert     `json:"parsed,omitempty"`
}

// ParsedCert contains parsed certificate fields
type ParsedCert struct {
	Subject            *DistinguishedName `json:"subject,omitempty"`
	Issuer             *DistinguishedName `json:"issuer,omitempty"`
	SerialNumber       string             `json:"serial_number,omitempty"`
	ValidityNotBefore  string             `json:"validity_not_before,omitempty"`
	ValidityNotAfter   string             `json:"validity_not_after,omitempty"`
	SignatureAlgorithm string             `json:"signature_algorithm,omitempty"`
	SubjectAltNames    *SubjectAltNames   `json:"subject_alt_name,omitempty"`
}

// DistinguishedName represents X.509 DN fields
type DistinguishedName struct {
	CommonName         []string `json:"common_name,omitempty"`
	Organization       []string `json:"organization,omitempty"`
	OrganizationalUnit []string `json:"organizational_unit,omitempty"`
	Country            []string `json:"country,omitempty"`
}

// SubjectAltNames contains SAN entries
type SubjectAltNames struct {
	DNSNames []string `json:"dns_names,omitempty"`
	IPAddrs  []string `json:"ip_addresses,omitempty"`
}

// ServerHello contains TLS negotiation info
type ServerHello struct {
	Version     uint16 `json:"version,omitempty"`
	CipherSuite uint16 `json:"cipher_suite,omitempty"`
}

// Protocol-specific result structs

// HTTPResult contains HTTP probe results
type HTTPResult struct {
	Response *HTTPResponse `json:"response,omitempty"`
}

// HTTPResponse contains HTTP response data
type HTTPResponse struct {
	StatusCode    int               `json:"status_code,omitempty"`
	StatusLine    string            `json:"status_line,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	BodySHA256    string            `json:"body_sha256,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	Protocol      map[string]interface{} `json:"protocol,omitempty"`
}

// SMTPResult contains SMTP probe results
type SMTPResult struct {
	Banner    string   `json:"banner,omitempty"`
	EHLO      string   `json:"ehlo,omitempty"`
	HELO      string   `json:"helo,omitempty"`
	StartTLS  string   `json:"starttls,omitempty"`
	TLS       *TLSLog  `json:"tls,omitempty"`
}

// FTPResult contains FTP probe results
type FTPResult struct {
	Banner   string  `json:"banner,omitempty"`
	AuthTLS  string  `json:"auth_tls,omitempty"`
	TLS      *TLSLog `json:"tls,omitempty"`
}

// SSHResult contains SSH probe results
type SSHResult struct {
	ServerID         *SSHServerID `json:"server_id,omitempty"`
	AlgorithmSelection map[string]interface{} `json:"algorithm_selection,omitempty"`
}

// SSHServerID contains SSH server identification
type SSHServerID struct {
	Raw             string `json:"raw,omitempty"`
	Version         string `json:"version,omitempty"`
	SoftwareVersion string `json:"software_version,omitempty"`
	Comment         string `json:"comment,omitempty"`
}

// MySQLResult contains MySQL probe results
type MySQLResult struct {
	ProtocolVersion int     `json:"protocol_version,omitempty"`
	ServerVersion   string  `json:"server_version,omitempty"`
	ConnectionID    uint32  `json:"connection_id,omitempty"`
	AuthPluginName  string  `json:"auth_plugin_name,omitempty"`
	TLS             *TLSLog `json:"tls,omitempty"`
}

// PostgresResult contains PostgreSQL probe results
type PostgresResult struct {
	SupportedVersions string  `json:"supported_versions,omitempty"`
	ProtocolError     string  `json:"protocol_error,omitempty"`
	StartupError      string  `json:"startup_error,omitempty"`
	IsSSL             bool    `json:"is_ssl,omitempty"`
	TLS               *TLSLog `json:"tls,omitempty"`
}

// RedisResult contains Redis probe results
type RedisResult struct {
	Ping     string `json:"ping,omitempty"`
	Info     string `json:"info,omitempty"`
	AuthRequired bool `json:"auth_required,omitempty"`
}

// IMAPResult contains IMAP probe results
type IMAPResult struct {
	Banner   string  `json:"banner,omitempty"`
	StartTLS string  `json:"starttls,omitempty"`
	TLS      *TLSLog `json:"tls,omitempty"`
}

// POP3Result contains POP3 probe results
type POP3Result struct {
	Banner   string  `json:"banner,omitempty"`
	StartTLS string  `json:"starttls,omitempty"`
	TLS      *TLSLog `json:"tls,omitempty"`
}

// TelnetResult contains Telnet probe results
type TelnetResult struct {
	Banner string `json:"banner,omitempty"`
}

// getZgrabModule returns the zgrab2 module name for a port
func getZgrabModule(port int) string {
	switch port {
	case 21:
		return "ftp"
	case 22:
		return "ssh"
	case 23:
		return "telnet"
	case 25, 465, 587:
		return "smtp"
	case 80, 8080, 8000, 8888:
		return "http"
	case 110, 995:
		return "pop3"
	case 143, 993:
		return "imap"
	case 443, 8443:
		return "http" // with --use-https flag
	case 3306:
		return "mysql"
	case 5432:
		return "postgres"
	case 6379:
		return "redis"
	case 27017:
		return "mongodb"
	default:
		return "banner" // Generic banner grab
	}
}

// FingerprintHost uses zgrab2 for enhanced fingerprinting
func (z *ZgrabFingerprinter) FingerprintHost(ctx context.Context, ip string, ports []int) map[int]ServiceInfo {
	results := make(map[int]ServiceInfo)

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		info := z.fingerprintPort(ctx, ip, port)
		results[port] = info
	}

	return results
}

func (z *ZgrabFingerprinter) fingerprintPort(ctx context.Context, ip string, port int) ServiceInfo {
	module := getZgrabModule(port)

	// Build zgrab2 command
	args := []string{module, "-p", fmt.Sprintf("%d", port)}

	// Add module-specific flags
	switch module {
	case "http":
		if port == 443 || port == 8443 {
			args = append(args, "--use-https")
		}
		args = append(args, "--max-redirects", "3")
	case "smtp":
		args = append(args, "--send-ehlo", "--ehlo-domain", "scanner.local")
		if port == 465 {
			args = append(args, "--smtps")
		} else {
			args = append(args, "--starttls")
		}
	case "ftp":
		args = append(args, "--authtls")
	case "imap":
		if port == 993 {
			args = append(args, "--imaps")
		} else {
			args = append(args, "--starttls")
		}
	case "pop3":
		if port == 995 {
			args = append(args, "--pop3s")
		} else {
			args = append(args, "--starttls")
		}
	case "mysql":
		// Default options are fine
	case "postgres":
		// Default options are fine
	case "redis":
		// Default options are fine
	case "banner":
		// Generic banner grab with probe
		args = append(args, "--probe", "\\x00", "--max-read-size", "4096")
	}

	// Execute zgrab2
	result, err := z.runZgrab(ctx, ip, args)
	if err != nil {
		// Fall back to native fingerprinting
		return z.Fallback.fingerprintPort(ctx, ip, port)
	}

	// Parse zgrab2 result
	info := z.parseZgrabResult(result, module, port)

	// Ensure we have a service name
	if info.ServiceName == "" {
		info.ServiceName = getDefaultServiceName(port)
	}

	return info
}

func (z *ZgrabFingerprinter) runZgrab(ctx context.Context, ip string, args []string) (*ZgrabResult, error) {
	ctx, cancel := context.WithTimeout(ctx, z.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zgrab2", args...)
	cmd.Stdin = strings.NewReader(ip + "\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("zgrab2 error: %v, stderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result ZgrabResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse zgrab2 output: %v", err)
	}

	return &result, nil
}

func (z *ZgrabFingerprinter) parseZgrabResult(result *ZgrabResult, module string, port int) ServiceInfo {
	info := ServiceInfo{
		Fingerprint: make(map[string]interface{}),
	}

	// Find the module result
	modResult, ok := result.Data[module]
	if !ok || modResult.Status != "success" {
		return info
	}

	info.Fingerprint["zgrab_status"] = modResult.Status
	info.Fingerprint["protocol"] = module

	// Parse protocol-specific results
	switch module {
	case "http":
		var httpRes HTTPResult
		if err := json.Unmarshal(modResult.Result, &httpRes); err == nil && httpRes.Response != nil {
			info.ServiceName = "http"
			if port == 443 || port == 8443 {
				info.ServiceName = "https"
			}

			resp := httpRes.Response
			if resp.StatusCode > 0 {
				info.Fingerprint["status_code"] = resp.StatusCode
			}
			if resp.StatusLine != "" {
				info.Banner = resp.StatusLine
			}
			if server, ok := resp.Headers["server"]; ok {
				info.ServiceVersion = server
			}
			if resp.Headers != nil {
				info.Fingerprint["headers"] = resp.Headers
			}
			// Extract title from body
			if resp.Body != "" {
				if title := extractTitle(resp.Body); title != "" {
					info.Fingerprint["title"] = title
				}
			}
		}

	case "smtp":
		var smtpRes SMTPResult
		if err := json.Unmarshal(modResult.Result, &smtpRes); err == nil {
			info.ServiceName = "smtp"
			if smtpRes.Banner != "" {
				info.Banner = sanitizeBanner(smtpRes.Banner)
				info.ServiceVersion = extractVersion(smtpRes.Banner)
			}
			if smtpRes.EHLO != "" {
				info.Fingerprint["ehlo"] = smtpRes.EHLO
				// Parse EHLO capabilities
				caps := parseEHLOCapabilities(smtpRes.EHLO)
				if len(caps) > 0 {
					info.Fingerprint["capabilities"] = caps
				}
			}
			if smtpRes.StartTLS != "" {
				info.Fingerprint["starttls"] = true
			}
			z.extractTLSInfo(&info, smtpRes.TLS)
		}

	case "ftp":
		var ftpRes FTPResult
		if err := json.Unmarshal(modResult.Result, &ftpRes); err == nil {
			info.ServiceName = "ftp"
			if ftpRes.Banner != "" {
				info.Banner = sanitizeBanner(ftpRes.Banner)
				info.ServiceVersion = extractVersion(ftpRes.Banner)
			}
			if ftpRes.AuthTLS != "" {
				info.Fingerprint["auth_tls"] = true
			}
			z.extractTLSInfo(&info, ftpRes.TLS)
		}

	case "ssh":
		var sshRes SSHResult
		if err := json.Unmarshal(modResult.Result, &sshRes); err == nil {
			info.ServiceName = "ssh"
			if sshRes.ServerID != nil {
				if sshRes.ServerID.Raw != "" {
					info.Banner = sanitizeBanner(sshRes.ServerID.Raw)
				}
				if sshRes.ServerID.SoftwareVersion != "" {
					info.ServiceVersion = sshRes.ServerID.SoftwareVersion
				}
				if sshRes.ServerID.Version != "" {
					info.Fingerprint["protocol_version"] = sshRes.ServerID.Version
				}
			}
			if sshRes.AlgorithmSelection != nil {
				info.Fingerprint["algorithms"] = sshRes.AlgorithmSelection
			}
		}

	case "mysql":
		var mysqlRes MySQLResult
		if err := json.Unmarshal(modResult.Result, &mysqlRes); err == nil {
			info.ServiceName = "mysql"
			if mysqlRes.ServerVersion != "" {
				info.ServiceVersion = mysqlRes.ServerVersion
				info.Banner = fmt.Sprintf("MySQL %s", mysqlRes.ServerVersion)
			}
			if mysqlRes.ProtocolVersion > 0 {
				info.Fingerprint["protocol_version"] = mysqlRes.ProtocolVersion
			}
			if mysqlRes.AuthPluginName != "" {
				info.Fingerprint["auth_plugin"] = mysqlRes.AuthPluginName
			}
			z.extractTLSInfo(&info, mysqlRes.TLS)
		}

	case "postgres":
		var pgRes PostgresResult
		if err := json.Unmarshal(modResult.Result, &pgRes); err == nil {
			info.ServiceName = "postgresql"
			if pgRes.IsSSL {
				info.Banner = "PostgreSQL (SSL supported)"
				info.Fingerprint["ssl_supported"] = true
			} else {
				info.Banner = "PostgreSQL"
			}
			if pgRes.SupportedVersions != "" {
				info.Fingerprint["supported_versions"] = pgRes.SupportedVersions
			}
			z.extractTLSInfo(&info, pgRes.TLS)
		}

	case "redis":
		var redisRes RedisResult
		if err := json.Unmarshal(modResult.Result, &redisRes); err == nil {
			info.ServiceName = "redis"
			if redisRes.AuthRequired {
				info.Banner = "Redis (authentication required)"
				info.Fingerprint["auth_required"] = true
			} else {
				info.Banner = "Redis"
			}
			if redisRes.Info != "" {
				// Extract version from INFO response
				if version := extractRedisVersion(redisRes.Info); version != "" {
					info.ServiceVersion = version
				}
			}
		}

	case "imap":
		var imapRes IMAPResult
		if err := json.Unmarshal(modResult.Result, &imapRes); err == nil {
			info.ServiceName = "imap"
			if imapRes.Banner != "" {
				info.Banner = sanitizeBanner(imapRes.Banner)
				info.ServiceVersion = extractVersion(imapRes.Banner)
			}
			if imapRes.StartTLS != "" {
				info.Fingerprint["starttls"] = true
			}
			z.extractTLSInfo(&info, imapRes.TLS)
		}

	case "pop3":
		var pop3Res POP3Result
		if err := json.Unmarshal(modResult.Result, &pop3Res); err == nil {
			info.ServiceName = "pop3"
			if pop3Res.Banner != "" {
				info.Banner = sanitizeBanner(pop3Res.Banner)
				info.ServiceVersion = extractVersion(pop3Res.Banner)
			}
			if pop3Res.StartTLS != "" {
				info.Fingerprint["starttls"] = true
			}
			z.extractTLSInfo(&info, pop3Res.TLS)
		}

	case "telnet":
		var telnetRes TelnetResult
		if err := json.Unmarshal(modResult.Result, &telnetRes); err == nil {
			info.ServiceName = "telnet"
			if telnetRes.Banner != "" {
				info.Banner = sanitizeBanner(telnetRes.Banner)
			}
		}

	case "banner":
		// Generic banner result
		var bannerRes map[string]interface{}
		if err := json.Unmarshal(modResult.Result, &bannerRes); err == nil {
			if banner, ok := bannerRes["banner"].(string); ok && banner != "" {
				info.Banner = sanitizeBanner(banner)
				info.ServiceName = guessServiceFromBanner(banner, port)
			}
		}
	}

	return info
}

func (z *ZgrabFingerprinter) extractTLSInfo(info *ServiceInfo, tls *TLSLog) {
	if tls == nil || tls.HandshakeLog == nil {
		return
	}

	hl := tls.HandshakeLog
	tlsInfo := make(map[string]interface{})

	if hl.ServerHello != nil {
		tlsInfo["version"] = hl.ServerHello.Version
		tlsInfo["cipher_suite"] = hl.ServerHello.CipherSuite
	}

	if hl.ServerCertificates != nil && hl.ServerCertificates.Certificate != nil {
		cert := hl.ServerCertificates.Certificate
		if cert.Parsed != nil {
			certInfo := make(map[string]interface{})
			p := cert.Parsed

			if p.Subject != nil && len(p.Subject.CommonName) > 0 {
				certInfo["subject_cn"] = p.Subject.CommonName[0]
			}
			if p.Issuer != nil && len(p.Issuer.CommonName) > 0 {
				certInfo["issuer_cn"] = p.Issuer.CommonName[0]
			}
			if p.ValidityNotBefore != "" {
				certInfo["valid_from"] = p.ValidityNotBefore
			}
			if p.ValidityNotAfter != "" {
				certInfo["valid_until"] = p.ValidityNotAfter
			}
			if p.SubjectAltNames != nil {
				if len(p.SubjectAltNames.DNSNames) > 0 {
					certInfo["san_dns"] = p.SubjectAltNames.DNSNames
				}
			}
			if p.SignatureAlgorithm != "" {
				certInfo["signature_algorithm"] = p.SignatureAlgorithm
			}

			tlsInfo["certificate"] = certInfo
		}

		// Count chain certificates
		if hl.ServerCertificates.Chain != nil {
			tlsInfo["chain_length"] = len(hl.ServerCertificates.Chain)
		}
	}

	if len(tlsInfo) > 0 {
		info.Fingerprint["tls"] = tlsInfo
	}
}

// Helper functions

func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	title := html[start : start+end]
	return strings.TrimSpace(title)
}

func parseEHLOCapabilities(ehlo string) []string {
	var caps []string
	lines := strings.Split(ehlo, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 4 && (line[3] == '-' || line[3] == ' ') {
			cap := strings.TrimSpace(line[4:])
			if cap != "" && !strings.HasPrefix(strings.ToLower(cap), "250") {
				caps = append(caps, cap)
			}
		}
	}
	return caps
}

func extractRedisVersion(info string) string {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "redis_version:") {
			return strings.TrimPrefix(line, "redis_version:")
		}
	}
	return ""
}
