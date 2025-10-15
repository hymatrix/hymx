package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	serverSchema "github.com/hymatrix/hymx/server/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

// ========================================
// Send Method Redirect Tests
// ========================================

// TestSendRedirectHandling tests the Send method's 308 redirect functionality
func TestSendRedirectHandling(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Return a successful response
		response := serverSchema.Response{
			Id:      "test-message-id",
			Message: "success",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer successServer.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   successServer.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Test data
	testData := []byte(`{"test": "data"}`)

	// Call Send method
	response, redirectedURL, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify redirected URL is set
	if redirectedURL == "" {
		t.Errorf("Expected redirected URL to be set, got empty string")
	}

	// Verify response is not nil
	if response == nil {
		t.Fatalf("Expected non-nil response, got nil")
	}

	// Verify response
	if response.Id != "test-message-id" {
		t.Errorf("Expected Id 'test-message-id', got '%s'", response.Id)
	}
	if response.Message != "success" {
		t.Errorf("Expected Message 'success', got '%s'", response.Message)
	}

	t.Log("✅ Send method successfully handled 308 redirect")
}

// TestSendRedirectWithFailedNodes tests Send method when all alternative nodes fail
func TestSendRedirectWithFailedNodes(t *testing.T) {
	// Create a mock failed server (alternative node)
	failedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer failedServer.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with failed alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   failedServer.URL,
			},
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   "http://invalid-node:9999",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Test data
	testData := []byte(`{"test": "data"}`)

	// Call Send method
	response, redirectedURL, err := client.Send(testData)

	// Should return error since all nodes failed (308 response)
	if err == nil {
		t.Fatal("Expected error when all nodes fail, but got nil")
	}

	// The response should be nil since we can't parse a successful response from 308
	if response != nil {
		t.Errorf("Expected nil response when all nodes fail, got %+v", response)
	}

	// Redirected URL should be empty when all nodes fail
	if redirectedURL != "" {
		t.Errorf("Expected empty redirected URL when all nodes fail, got %s", redirectedURL)
	}

	t.Log("✅ Send method correctly handled failed alternative nodes")
}

// TestSendWithoutRedirect tests Send method with normal successful response
func TestSendWithoutRedirect(t *testing.T) {
	// Create a mock successful server
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}
		expectedBody := `{"test": "data"}`
		if string(body) != expectedBody {
			t.Errorf("Expected body '%s', got '%s'", expectedBody, string(body))
		}

		// Return successful response
		response := serverSchema.Response{
			Id:      "direct-message-id",
			Message: "direct-success",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer successServer.Close()

	// Create client pointing to success server
	client := NewClient(successServer.URL)

	// Test data
	testData := []byte(`{"test": "data"}`)

	// Call Send method
	response, redirectedURL, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Redirected URL should be empty when no redirect occurs
	if redirectedURL != "" {
		t.Errorf("Expected empty redirected URL when no redirect occurs, got %s", redirectedURL)
	}

	// Verify response is not nil
	if response == nil {
		t.Fatalf("Expected non-nil response, got nil")
	}

	// Verify response
	if response.Id != "direct-message-id" {
		t.Errorf("Expected Id 'direct-message-id', got '%s'", response.Id)
	}
	if response.Message != "direct-success" {
		t.Errorf("Expected Message 'direct-success', got '%s'", response.Message)
	}

	t.Log("✅ Send method works correctly without redirect")
}

// TestSendRedirectPreservesRequestBody tests that request body is preserved during redirect
func TestSendRedirectPreservesRequestBody(t *testing.T) {
	var receivedBody []byte

	// Create a mock successful server that captures the request body
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			return
		}
		receivedBody = body

		// Return successful response
		response := serverSchema.Response{
			Id:      "body-test-id",
			Message: "body-preserved",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer successServer.Close()

	// Create a mock redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   successServer.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client
	client := NewClient(redirectServer.URL)

	// Test with complex JSON data
	testData := []byte(`{"complex": {"nested": "data", "array": [1, 2, 3]}, "message": "test redirect body preservation"}`)

	// Call Send method
	_, redirectedURL, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify redirected URL is set
	if redirectedURL == "" {
		t.Errorf("Expected redirected URL to be set, got empty string")
	}

	// Verify the request body was preserved
	if !bytes.Equal(receivedBody, testData) {
		t.Errorf("Request body not preserved during redirect.\nExpected: %s\nReceived: %s", string(testData), string(receivedBody))
	}

	t.Log("✅ Send method correctly preserves request body during redirect")
}

// ========================================
// GetResult Method Redirect Tests
// ========================================

// TestGetResultRedirectHandling tests the GetResult method's 308 redirect functionality
func TestGetResultRedirectHandling(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify the URL path contains the expected process ID and message ID
		expectedPath := "/result/test-process-id/test-message-id"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}

		// Return a successful result response
		result := vmmSchema.Result{
			ItemId:      "test-item-id",
			FromProcess: "test-process-id",
			Output:      "test-output",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer successServer.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   successServer.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Call GetResult method
	result, err := client.GetResult("test-process-id", "test-message-id")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify result
	if result.ItemId != "test-item-id" {
		t.Errorf("Expected ItemId 'test-item-id', got '%s'", result.ItemId)
	}
	if result.FromProcess != "test-process-id" {
		t.Errorf("Expected FromProcess 'test-process-id', got '%s'", result.FromProcess)
	}
	if result.Output != "test-output" {
		t.Errorf("Expected Output 'test-output', got '%v'", result.Output)
	}

	t.Log("✅ GetResult method successfully handled 308 redirect")
}

// TestGetResultRedirectWithFailedNodes tests GetResult when all alternative nodes fail
func TestGetResultRedirectWithFailedNodes(t *testing.T) {
	// Create mock failed servers (alternative nodes)
	failedServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failedServer1.Close()

	failedServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failedServer2.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with failed alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   failedServer1.URL,
			},
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   failedServer2.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Call GetResult method - should fail since all alternative nodes fail
	_, err := client.GetResult("test-process-id", "test-message-id")
	if err == nil {
		t.Fatal("Expected GetResult to fail when all alternative nodes fail, but it succeeded")
	}

	t.Logf("✅ GetResult correctly failed when all alternative nodes failed: %v", err)
}

// TestGetResultWithoutRedirect tests GetResult method with normal successful response
func TestGetResultWithoutRedirect(t *testing.T) {
	// Create a mock successful server
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify URL path
		expectedPath := "/result/direct-process-id/direct-message-id"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}

		// Return successful result response
		result := vmmSchema.Result{
			ItemId:      "direct-item-id",
			FromProcess: "direct-process-id",
			Output:      "direct-output",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer successServer.Close()

	// Create client pointing to success server
	client := NewClient(successServer.URL)

	// Call GetResult method
	result, err := client.GetResult("direct-process-id", "direct-message-id")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify result
	if result.ItemId != "direct-item-id" {
		t.Errorf("Expected ItemId 'direct-item-id', got '%s'", result.ItemId)
	}
	if result.FromProcess != "direct-process-id" {
		t.Errorf("Expected FromProcess 'direct-process-id', got '%s'", result.FromProcess)
	}
	if result.Output != "direct-output" {
		t.Errorf("Expected Output 'direct-output', got '%s'", result.Output)
	}

	t.Log("✅ GetResult method works correctly without redirect")
}

// TestGetResultRedirectPreservesURLPath tests that URL path is preserved during redirect
func TestGetResultRedirectPreservesURLPath(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the URL path is preserved
		expectedPath := "/result/path-process-id/path-message-id"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}

		// Return successful result response
		result := vmmSchema.Result{
			ItemId:      "path-test-id",
			FromProcess: "path-process-id",
			Output:      "path-preserved",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer successServer.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   successServer.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Call GetResult method
	result, err := client.GetResult("path-process-id", "path-message-id")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify result
	if result.ItemId != "path-test-id" {
		t.Errorf("Expected ItemId 'path-test-id', got '%s'", result.ItemId)
	}

	t.Log("✅ GetResult method preserved URL path during redirect")
}

// TestGetResultRedirectWithMultipleNodes tests GetResult with multiple alternative nodes
func TestGetResultRedirectWithMultipleNodes(t *testing.T) {
	// Create a mock failed server (first alternative node)
	failedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failedServer.Close()

	// Create a mock successful server (second alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := vmmSchema.Result{
			ItemId:      "multi-node-item-id",
			FromProcess: "multi-node-process-id",
			Output:      "multi-node-success",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer successServer.Close()

	// Create a mock redirect server (main node)
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 308 redirect with multiple alternative nodes
		redirectResp := []registrySchema.Node{
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   failedServer.URL,
			},
			{
				AccId: "",
				Name:  "",
				Role:  "",
				Desc:  "",
				URL:   successServer.URL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(308)
		json.NewEncoder(w).Encode(redirectResp)
	}))
	defer redirectServer.Close()

	// Create client pointing to redirect server
	client := NewClient(redirectServer.URL)

	// Call GetResult method - should succeed with second node
	result, err := client.GetResult("multi-node-process-id", "multi-node-message-id")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify result
	if result.ItemId != "multi-node-item-id" {
		t.Errorf("Expected ItemId 'multi-node-item-id', got '%s'", result.ItemId)
	}
	if result.FromProcess != "multi-node-process-id" {
		t.Errorf("Expected FromProcess 'multi-node-process-id', got '%s'", result.FromProcess)
	}

	t.Log("✅ GetResult method successfully handled redirect with multiple nodes")
}

// ========================================
// Test Main Function
// ========================================

func TestMain(m *testing.M) {
	fmt.Println("🧪 Running SDK Client Redirect Tests")
	fmt.Println("==========================================")
	result := m.Run()
	fmt.Println("==========================================")
	if result == 0 {
		fmt.Println("✅ All client redirect tests passed!")
	} else {
		fmt.Println("❌ Some tests failed")
	}
	fmt.Println()
}