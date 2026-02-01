package scanner

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ZmapResult represents a single result from scan
type ZmapResult struct {
	IP   string
	Port int
}

// ZmapScanner wraps zmap scanning functionality
type ZmapScanner struct {
	Networks    []string
	Rate        int           // packets per second
	Timeout     time.Duration // connection timeout for banner grabbing
	Interface   string        // network interface (optional)
}

// NewZmapScanner creates a new ZmapScanner instance
func NewZmapScanner(networks []string, rate int, timeoutSecs int) *ZmapScanner {
	if rate <= 0 {
		rate = 10000 // default packets per second for zmap
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 5
	}
	return &ZmapScanner{
		Networks: networks,
		Rate:     rate,
		Timeout:  time.Duration(timeoutSecs) * time.Second,
	}
}

// ScanPort scans a specific port across all configured networks using zmap
func (z *ZmapScanner) ScanPort(ctx context.Context, port int) ([]ZmapResult, error) {
	var allResults []ZmapResult

	for _, network := range z.Networks {
		results, err := z.scanNetworkPort(ctx, network, port)
		if err != nil {
			log.Printf("Warning: error scanning %s:%d: %v", network, port, err)
			continue
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// scanNetworkPort scans a single network for a specific port using zmap
func (z *ZmapScanner) scanNetworkPort(ctx context.Context, network string, port int) ([]ZmapResult, error) {
	// Create a whitelist file for the network (zmap requires this for private ranges)
	whitelistFile, err := os.CreateTemp("", "zmap-whitelist-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create whitelist file: %w", err)
	}
	defer os.Remove(whitelistFile.Name())

	if _, err := whitelistFile.WriteString(network + "\n"); err != nil {
		whitelistFile.Close()
		return nil, fmt.Errorf("failed to write whitelist: %w", err)
	}
	whitelistFile.Close()

	// Create an empty blacklist file (to allow scanning private networks)
	blacklistFile, err := os.CreateTemp("", "zmap-blacklist-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create blacklist file: %w", err)
	}
	defer os.Remove(blacklistFile.Name())
	blacklistFile.Close()

	// Build zmap command
	args := []string{
		"-p", strconv.Itoa(port),
		"-w", whitelistFile.Name(),
		"-b", blacklistFile.Name(), // empty blacklist to allow private ranges
		"-r", strconv.Itoa(z.Rate),
		"-o", "-",           // output to stdout
		"-f", "saddr",       // only output source address
		"--output-module=csv",
		"-q",                // quiet mode
		"--disable-syslog",
		"--cooldown-time=3", // reduce wait time after sending
	}

	if z.Interface != "" {
		args = append(args, "-i", z.Interface)
	}

	cmd := exec.CommandContext(ctx, "zmap", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	log.Printf("Running zmap command: zmap %s", strings.Join(args, " "))

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start zmap: %w", err)
	}

	// Read results
	var results []ZmapResult
	reader := csv.NewReader(stdout)

	// Skip header
	_, err = reader.Read()
	if err != nil && err != io.EOF {
		// No header or empty output is OK
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(record) > 0 && record[0] != "" {
			results = append(results, ZmapResult{
				IP:   strings.TrimSpace(record[0]),
				Port: port,
			})
		}
	}

	// Read stderr for any errors
	stderrBytes, _ := io.ReadAll(stderr)
	stderrStr := string(stderrBytes)

	if err := cmd.Wait(); err != nil {
		log.Printf("zmap stderr: %s", stderrStr)
		log.Printf("zmap error: %v", err)
		// Check if it's just a timeout or context cancellation
		if ctx.Err() != nil {
			return results, ctx.Err()
		}
		// Log stderr but don't fail if we got some results
		if len(results) == 0 && stderrStr != "" {
			return nil, fmt.Errorf("zmap failed: %s", stderrStr)
		}
	}

	return results, nil
}

// ScanPorts scans multiple ports across all configured networks
func (z *ZmapScanner) ScanPorts(ctx context.Context, ports []int) (map[string][]int, error) {
	results := make(map[string][]int)

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		log.Printf("Scanning port %d across %d networks...", port, len(z.Networks))
		portResults, err := z.ScanPort(ctx, port)
		if err != nil {
			log.Printf("Error scanning port %d: %v", port, err)
			continue
		}

		log.Printf("Port %d: found %d hosts", port, len(portResults))

		for _, r := range portResults {
			results[r.IP] = append(results[r.IP], r.Port)
		}
	}

	return results, nil
}

// ScanAllPorts scans all 65535 ports using zmap (much faster than per-port)
func (z *ZmapScanner) ScanAllPorts(ctx context.Context) (map[string][]int, error) {
	results := make(map[string][]int)

	for _, network := range z.Networks {
		log.Printf("Scanning all ports on %s...", network)
		networkResults, err := z.scanNetworkAllPorts(ctx, network)
		if err != nil {
			log.Printf("Warning: error scanning %s: %v", network, err)
			continue
		}

		for ip, ports := range networkResults {
			results[ip] = append(results[ip], ports...)
		}
	}

	return results, nil
}

// scanNetworkAllPorts scans all ports on a network by iterating through all 65535 ports
// This version of zmap (2.1.1) doesn't support port ranges, so we scan individual ports
func (z *ZmapScanner) scanNetworkAllPorts(ctx context.Context, network string) (map[string][]int, error) {
	results := make(map[string][]int)

	// Scan all 65535 ports individually
	// Group into batches for logging
	totalPorts := 65535
	batchSize := 1000

	for batchStart := 1; batchStart <= totalPorts; batchStart += batchSize {
		batchEnd := batchStart + batchSize - 1
		if batchEnd > totalPorts {
			batchEnd = totalPorts
		}

		log.Printf("Scanning ports %d-%d on %s...", batchStart, batchEnd, network)

		for port := batchStart; port <= batchEnd; port++ {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			default:
			}

			portResults, err := z.scanNetworkPort(ctx, network, port)
			if err != nil {
				continue
			}

			for _, r := range portResults {
				results[r.IP] = append(results[r.IP], r.Port)
			}
		}
	}

	return results, nil
}

// CommonPorts returns a list of commonly scanned ports
func CommonPorts() []int {
	return []int{
		21,    // FTP
		22,    // SSH
		23,    // Telnet
		25,    // SMTP
		53,    // DNS
		80,    // HTTP
		110,   // POP3
		143,   // IMAP
		443,   // HTTPS
		445,   // SMB
		993,   // IMAPS
		995,   // POP3S
		1433,  // MSSQL
		1521,  // Oracle
		3306,  // MySQL
		3389,  // RDP
		5432,  // PostgreSQL
		5900,  // VNC
		6379,  // Redis
		8080,  // HTTP Alt
		8443,  // HTTPS Alt
		27017, // MongoDB
	}
}

// Zgrab2Scanner wraps zgrab2 for service fingerprinting
type Zgrab2Scanner struct {
	Timeout time.Duration
}

// NewZgrab2Scanner creates a new Zgrab2Scanner
func NewZgrab2Scanner(timeoutSecs int) *Zgrab2Scanner {
	if timeoutSecs <= 0 {
		timeoutSecs = 10
	}
	return &Zgrab2Scanner{
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}
}

// GrabBanner uses zgrab2 to grab service banner
func (z *Zgrab2Scanner) GrabBanner(ctx context.Context, ip string, port int, module string) (string, error) {
	args := []string{module}

	// Add module-specific options
	switch module {
	case "http":
		args = append(args, "--port", strconv.Itoa(port))
	case "tls":
		args = append(args, "--port", strconv.Itoa(port))
	case "ssh":
		args = append(args, "--port", strconv.Itoa(port))
	case "banner":
		args = append(args, "--port", strconv.Itoa(port))
	default:
		args = append(args, "--port", strconv.Itoa(port))
	}

	cmd := exec.CommandContext(ctx, "zgrab2", args...)
	cmd.Stdin = strings.NewReader(ip + "\n")

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GetModuleForPort returns the appropriate zgrab2 module for a port
func GetModuleForPort(port int) string {
	switch port {
	case 22:
		return "ssh"
	case 80, 8080, 8000, 8888:
		return "http"
	case 443, 8443:
		return "tls"
	case 21:
		return "ftp"
	case 25, 465, 587:
		return "smtp"
	case 110:
		return "pop3"
	case 143:
		return "imap"
	case 3306:
		return "mysql"
	case 5432:
		return "postgres"
	case 6379:
		return "redis"
	case 27017:
		return "mongodb"
	default:
		return "banner"
	}
}
