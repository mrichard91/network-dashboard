package scanner

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// TCPScanner wraps TCP connect scanning functionality
type TCPScanner struct {
	Networks []string
	Rate     int           // concurrent connections
	Timeout  time.Duration // connection timeout
}

// NewTCPScanner creates a new TCPScanner instance
func NewTCPScanner(networks []string, rate int, timeoutSecs int) *TCPScanner {
	if rate <= 0 {
		rate = 100 // default concurrent connections
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 5
	}
	return &TCPScanner{
		Networks: networks,
		Rate:     rate,
		Timeout:  time.Duration(timeoutSecs) * time.Second,
	}
}

// expandCIDR expands a CIDR notation to a list of IPs
func expandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast addresses for /24 and smaller
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ScanPort scans a specific port across all configured networks using TCP connect
func (t *TCPScanner) ScanPort(ctx context.Context, port int) ([]ZmapResult, error) {
	// Collect all IPs from all networks
	var allIPs []string
	for _, network := range t.Networks {
		ips, err := expandCIDR(network)
		if err != nil {
			log.Printf("Warning: failed to parse CIDR %s: %v", network, err)
			continue
		}
		allIPs = append(allIPs, ips...)
	}

	if len(allIPs) == 0 {
		return nil, fmt.Errorf("no valid IPs to scan")
	}

	var results []ZmapResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore for rate limiting
	sem := make(chan struct{}, t.Rate)

	for _, ip := range allIPs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // acquire

		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }() // release

			address := fmt.Sprintf("%s:%d", targetIP, port)
			conn, err := net.DialTimeout("tcp", address, t.Timeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				results = append(results, ZmapResult{IP: targetIP, Port: port})
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return results, nil
}

// PortScanCallback is called after each port is scanned with results
type PortScanCallback func(port int, results []ZmapResult)

// ScanPorts scans multiple ports across all configured networks
func (t *TCPScanner) ScanPorts(ctx context.Context, ports []int) (map[string][]int, error) {
	return t.ScanPortsWithCallback(ctx, ports, nil)
}

// ScanPortsWithCallback scans ports and calls callback after each port
func (t *TCPScanner) ScanPortsWithCallback(ctx context.Context, ports []int, callback PortScanCallback) (map[string][]int, error) {
	results := make(map[string][]int)

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		log.Printf("Scanning port %d across %d networks...", port, len(t.Networks))
		portResults, err := t.ScanPort(ctx, port)
		if err != nil {
			log.Printf("Error scanning port %d: %v", port, err)
			continue
		}

		log.Printf("Port %d: found %d hosts", port, len(portResults))

		for _, r := range portResults {
			results[r.IP] = append(results[r.IP], r.Port)
		}

		// Call callback with results for this port
		if callback != nil && len(portResults) > 0 {
			callback(port, portResults)
		}
	}

	return results, nil
}

// ScanAllPorts scans all 65535 ports using TCP connect
func (t *TCPScanner) ScanAllPorts(ctx context.Context) (map[string][]int, error) {
	results := make(map[string][]int)

	// Scan all 65535 ports
	totalPorts := 65535
	batchSize := 1000

	for batchStart := 1; batchStart <= totalPorts; batchStart += batchSize {
		batchEnd := batchStart + batchSize - 1
		if batchEnd > totalPorts {
			batchEnd = totalPorts
		}

		log.Printf("Scanning ports %d-%d across %d networks...", batchStart, batchEnd, len(t.Networks))

		// Generate port list for this batch
		var batchPorts []int
		for port := batchStart; port <= batchEnd; port++ {
			batchPorts = append(batchPorts, port)
		}

		batchResults, err := t.ScanPorts(ctx, batchPorts)
		if err != nil {
			log.Printf("Error scanning ports %d-%d: %v", batchStart, batchEnd, err)
			continue
		}

		for ip, ports := range batchResults {
			results[ip] = append(results[ip], ports...)
		}
	}

	return results, nil
}
