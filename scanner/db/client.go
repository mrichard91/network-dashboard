package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// APIClient handles communication with the API service
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScanResultPort represents a port in scan results
type ScanResultPort struct {
	PortNumber      int                    `json:"port_number"`
	Protocol        string                 `json:"protocol"`
	State           string                 `json:"state"`
	ServiceName     string                 `json:"service_name,omitempty"`
	ServiceVersion  string                 `json:"service_version,omitempty"`
	Banner          string                 `json:"banner,omitempty"`
	FingerprintData map[string]interface{} `json:"fingerprint_data,omitempty"`
}

// ScanResultHost represents a host in scan results
type ScanResultHost struct {
	IPAddress  string           `json:"ip_address"`
	Hostname   string           `json:"hostname,omitempty"`
	MACAddress string           `json:"mac_address,omitempty"`
	Ports      []ScanResultPort `json:"ports"`
}

// ScanResults represents the complete scan results
type ScanResults struct {
	ScanID uuid.UUID        `json:"scan_id"`
	Hosts  []ScanResultHost `json:"hosts"`
}

// SubmitResults sends scan results to the API
func (c *APIClient) SubmitResults(results *ScanResults) error {
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	url := fmt.Sprintf("%s/api/scan/results", c.BaseURL)
	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to submit results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck checks if the API is available
func (c *APIClient) HealthCheck() error {
	url := fmt.Sprintf("%s/api/health", c.BaseURL)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
