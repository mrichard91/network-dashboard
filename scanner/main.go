package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"network-scanner/config"
	"network-scanner/db"
	"network-scanner/scanner"
)

var (
	scanMutex    sync.Mutex
	isScanning   bool
	lastScanTime time.Time
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Network Scanner starting...")

	// Load configuration
	configPath := getEnv("CONFIG_PATH", "/etc/scanner/config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Warning: failed to load config from %s: %v", configPath, err)
		log.Println("Using default configuration")
		cfg = config.Default()
	}

	// Allow environment variables to override config
	if apiURL := os.Getenv("API_URL"); apiURL != "" {
		cfg.APIURL = apiURL
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Networks: %v", cfg.Networks)
	log.Printf("  Scan all ports: %v", cfg.ScanAllPorts)
	if !cfg.ScanAllPorts {
		log.Printf("  Ports: %v", cfg.Ports)
	}
	log.Printf("  Schedule: %s", cfg.Schedule)
	log.Printf("  Scanner mode: %s", cfg.ScannerMode)
	log.Printf("  Rate: %d", cfg.Rate)
	log.Printf("  Timeout: %ds", cfg.Timeout)
	log.Printf("  Interface: %s", cfg.Interface)
	log.Printf("  API URL: %s", cfg.APIURL)

	// Create scanner components based on mode
	var zmapScanner *scanner.ZmapScanner
	var tcpScanner *scanner.TCPScanner
	useZmap := cfg.ScannerMode == "zmap"

	if useZmap {
		zmapScanner = scanner.NewZmapScanner(cfg.Networks, cfg.Rate, cfg.Timeout)
		if cfg.Interface != "" {
			zmapScanner.Interface = cfg.Interface
		}
	} else {
		tcpScanner = scanner.NewTCPScanner(cfg.Networks, cfg.Rate, cfg.Timeout)
	}

	fingerprinter := scanner.NewZgrabFingerprinter()
	apiClient := db.NewAPIClient(cfg.APIURL)

	// Wait for API to be ready
	log.Println("Waiting for API to be ready...")
	for i := 0; i < 30; i++ {
		if err := apiClient.HealthCheck(); err == nil {
			log.Println("API is ready")
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Create the scan function
	runScan := func() {
		scanMutex.Lock()
		if isScanning {
			scanMutex.Unlock()
			log.Println("Scan already in progress, skipping...")
			return
		}
		isScanning = true
		scanMutex.Unlock()

		defer func() {
			scanMutex.Lock()
			isScanning = false
			lastScanTime = time.Now()
			scanMutex.Unlock()
		}()

		log.Println("Starting network scan...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour) // Allow more time for full port scans
		defer cancel()

		scanID := uuid.New()
		log.Printf("Scan ID: %s", scanID)

		var hostPorts map[string][]int
		var err error

		scannerName := "tcp"
		if useZmap {
			scannerName = "zmap"
		}

		if cfg.ScanAllPorts {
			log.Printf("Scanning ALL ports (1-65535) on networks %v using %s", cfg.Networks, scannerName)
			if useZmap {
				hostPorts, err = zmapScanner.ScanAllPorts(ctx)
			} else {
				hostPorts, err = tcpScanner.ScanAllPorts(ctx)
			}
		} else {
			ports := cfg.Ports
			if len(ports) == 0 {
				ports = scanner.CommonPorts()
			}
			log.Printf("Scanning ports %v on networks %v using %s", ports, cfg.Networks, scannerName)
			if useZmap {
				hostPorts, err = zmapScanner.ScanPorts(ctx, ports)
			} else {
				hostPorts, err = tcpScanner.ScanPorts(ctx, ports)
			}
		}

		if err != nil {
			log.Printf("Error during scan: %v", err)
			return
		}

		log.Printf("Found %d hosts with open ports", len(hostPorts))

		// Fingerprint services for each host and submit immediately
		for ip, openPorts := range hostPorts {
			log.Printf("Fingerprinting %s (%d ports)", ip, len(openPorts))

			host := db.ScanResultHost{
				IPAddress: ip,
				Ports:     make([]db.ScanResultPort, 0, len(openPorts)),
			}

			// Get service info using our native Go fingerprinter
			serviceInfo := fingerprinter.FingerprintHost(ctx, ip, openPorts)

			for _, port := range openPorts {
				portResult := db.ScanResultPort{
					PortNumber: port,
					Protocol:   "tcp",
					State:      "open",
				}

				if info, ok := serviceInfo[port]; ok {
					portResult.ServiceName = info.ServiceName
					portResult.ServiceVersion = info.ServiceVersion
					portResult.Banner = info.Banner
					portResult.FingerprintData = info.Fingerprint
				}

				host.Ports = append(host.Ports, portResult)
			}

			results := &db.ScanResults{
				ScanID: scanID,
				Hosts:  []db.ScanResultHost{host},
			}

			if err := apiClient.SubmitResults(results); err != nil {
				log.Printf("Failed to submit results for host %s: %v", ip, err)
			}
		}

		log.Println("Scan completed successfully")
	}

	// Set up HTTP server for triggering scans
	http.HandleFunc("/trigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		scanMutex.Lock()
		scanning := isScanning
		scanMutex.Unlock()

		if scanning {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "already_running",
				"message": "Scan already in progress",
			})
			return
		}

		// Start scan in background
		go runScan()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "started",
			"message": "Scan started",
		})
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		scanMutex.Lock()
		scanning := isScanning
		lastScan := lastScanTime
		scanMutex.Unlock()

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"is_scanning": scanning,
		}
		if !lastScan.IsZero() {
			response["last_scan_time"] = lastScan.Format(time.RFC3339)
		}
		json.NewEncoder(w).Encode(response)
	})

	// Start HTTP server
	go func() {
		log.Println("Starting HTTP server on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Run initial scan
	runScan()

	// Set up cron scheduler
	c := cron.New()
	_, err = c.AddFunc(cfg.Schedule, runScan)
	if err != nil {
		log.Fatalf("Failed to set up cron: %v", err)
	}
	c.Start()
	log.Printf("Scheduled scans with interval: %s", cfg.Schedule)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	c.Stop()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
