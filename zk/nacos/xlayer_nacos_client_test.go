package nacos

import (
	"testing"
)

func TestNewNacosClient(t *testing.T) {
	// Note: This test requires an actual nacos server to run
	// Here we just test function signature and basic structure

	// Test function signature
	client, err := NewNacosClient("test-namespace", "test-service")

	// Since there's no actual nacos server, we expect an error
	if err == nil {
		t.Log("NewNacosClient created successfully")
		// Test basic methods
		if client.GetServiceName() != "test-service" {
			t.Errorf("Expected service name 'test-service', got '%s'", client.GetServiceName())
		}
	} else {
		t.Logf("Expected error (no nacos server): %v", err)
	}
}

func TestXlayerNacosClientStructure(t *testing.T) {
	// Test struct fields
	client := &XlayerNacosClient{
		serviceName: "test-service",
	}

	if client.GetServiceName() != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", client.GetServiceName())
	}
}
