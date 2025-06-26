package nacos

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ledgerwatch/log/v3"
)

// XlayerNacosClient struct for managing Nacos client and instances
type XlayerNacosClient struct {
	baseURL     string
	serviceName string
	httpClient  *http.Client
}

// NewNacosClient creates a nacos NamingClient based on the specified namespace
// Uses NamingClient and specified service name to call SelectOneHealthyInstance to get an instance
// Stores NamingClient and instance in XlayerNacosClient struct and returns it
func NewNacosClient(serviceName string) (*XlayerNacosClient, error) {
	client := &XlayerNacosClient{
		serviceName: serviceName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	log.Info("Created XlayerNacosClient", "service", serviceName)
	return client, nil
}

// Http sends HTTP request based on specified method (GET, POST, PUT, etc.), API path, and body
// If request fails, uses NamingClient.SelectOneHealthyInstance to get a new instance, updates current instance, and retries the request
func (c *XlayerNacosClient) Http(method string, apiPath string, body []byte, headers map[string]string) ([]byte, error) {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 || c.baseURL == "" {
			log.Info("Retrying request with new instance", "attempt", attempt, "service", c.serviceName)

			url, err := GetOneURL(c.serviceName)
			if err != nil {
				lastErr = fmt.Errorf("failed to get one url: %w", err)
				continue
			}
			// Build request URL
			c.baseURL = fmt.Sprintf("http://%s", url)
		}

		queryURL := fmt.Sprintf("%s/%s", c.baseURL, apiPath)

		// Create request
		var req *http.Request
		var err error

		if body != nil {
			req, err = http.NewRequest(method, queryURL, bytes.NewBuffer(body))
		} else {
			req, err = http.NewRequest(method, queryURL, nil)
		}

		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set request headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Send request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request to %s: %w", queryURL, err)
			continue
		}
		defer resp.Body.Close()

		// Read response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Check response status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Debug("Request successful", "method", method, "url", queryURL, "status", resp.StatusCode)
			return respBody, nil
		}

		lastErr = fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// Get sends GET request
func (c *XlayerNacosClient) Get(apiPath string, headers map[string]string) ([]byte, error) {
	return c.Http("GET", apiPath, nil, headers)
}

// Put sends PUT request
func (c *XlayerNacosClient) Put(apiPath string, body []byte, headers map[string]string) ([]byte, error) {
	return c.Http("PUT", apiPath, body, headers)
}

// Post sends POST request
func (c *XlayerNacosClient) Post(apiPath string, body []byte, headers map[string]string) ([]byte, error) {
	return c.Http("POST", apiPath, body, headers)
}

// Delete sends DELETE request
func (c *XlayerNacosClient) Delete(apiPath string, headers map[string]string) ([]byte, error) {
	return c.Http("DELETE", apiPath, nil, headers)
}

// GetServiceName returns service name
func (c *XlayerNacosClient) GetServiceName() string {
	return c.serviceName
}
