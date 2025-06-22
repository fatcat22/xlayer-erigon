package nacos

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ledgerwatch/log/v3"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// XlayerNacosClient struct for managing Nacos client and instances
type XlayerNacosClient struct {
	namingClient interface{}
	instance     *model.Instance
	serviceName  string
	httpClient   *http.Client
}

// NewNacosClient creates a nacos NamingClient based on the specified namespace
// Uses NamingClient and specified service name to call SelectOneHealthyInstance to get an instance
// Stores NamingClient and instance in XlayerNacosClient struct and returns it
func NewNacosClient(namespace string, serviceName string) (*XlayerNacosClient, error) {
	namingClient, err := clients.CreateNamingClient(map[string]interface{}{
		"serverConfigs": constant.ServerConfig{},
		"clientConfig": constant.ClientConfig{
			TimeoutMs:           defaultTimeoutMs,
			ListenInterval:      defaultListenInterval,
			NotLoadCacheAtStart: true,
			NamespaceId:         namespace,
			LogDir:              "/dev/null",
			LogLevel:            "error",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos naming client: %w", err)
	}

	// Get a healthy instance
	instance, err := namingClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to select healthy instance for service %s: %w", serviceName, err)
	}

	client := &XlayerNacosClient{
		namingClient: namingClient,
		instance:     instance,
		serviceName:  serviceName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	log.Info("Created XlayerNacosClient", "service", serviceName, "instance", fmt.Sprintf("%s:%d", instance.Ip, instance.Port))
	return client, nil
}

// Http sends HTTP request based on specified method (GET, POST, PUT, etc.), API path, and body
// If request fails, uses NamingClient.SelectOneHealthyInstance to get a new instance, updates current instance, and retries the request
func (c *XlayerNacosClient) Http(method string, apiPath string, body []byte, headers map[string]string) ([]byte, error) {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Info("Retrying request with new instance", "attempt", attempt, "service", c.serviceName)

			// Get a new healthy instance
			if namingClient, ok := c.namingClient.(interface {
				SelectOneHealthyInstance(param vo.SelectOneHealthInstanceParam) (*model.Instance, error)
			}); ok {
				newInstance, err := namingClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
					ServiceName: c.serviceName,
				})
				if err != nil {
					lastErr = fmt.Errorf("failed to select new healthy instance: %w", err)
					continue
				}

				c.instance = newInstance
				log.Info("Updated instance", "new_instance", fmt.Sprintf("%s:%d", newInstance.Ip, newInstance.Port))
			}
		}

		// Build request URL
		url := fmt.Sprintf("http://%s:%d%s", c.instance.Ip, c.instance.Port, apiPath)

		// Create request
		var req *http.Request
		var err error

		if body != nil {
			req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
		} else {
			req, err = http.NewRequest(method, url, nil)
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
			lastErr = fmt.Errorf("failed to send request to %s: %w", url, err)
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
			log.Debug("Request successful", "method", method, "url", url, "status", resp.StatusCode)
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

// GetCurrentInstance returns current instance information
func (c *XlayerNacosClient) GetCurrentInstance() *model.Instance {
	return c.instance
}

// GetServiceName returns service name
func (c *XlayerNacosClient) GetServiceName() string {
	return c.serviceName
}
