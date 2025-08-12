package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
)

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
		redirectResp := nodeSchema.RedirectError{
			Nodes: []registrySchema.Node{
				{URL: successServer.URL},
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
	response, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
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
		redirectResp := nodeSchema.RedirectError{
			Nodes: []registrySchema.Node{
				{URL: failedServer.URL},
				{URL: "http://invalid-node:9999"},
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
	response, err := client.Send(testData)

	// Should not return error, but should get 308 response
	if err != nil {
		t.Fatalf("Send failed unexpectedly: %v", err)
	}

	// The response should be nil since we can't parse a successful response from 308
	if response != nil {
		t.Errorf("Expected nil response when all nodes fail, got %+v", response)
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
	response, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
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
		redirectResp := nodeSchema.RedirectError{
			Nodes: []registrySchema.Node{
				{URL: successServer.URL},
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
	_, err := client.Send(testData)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the request body was preserved
	if !bytes.Equal(receivedBody, testData) {
		t.Errorf("Request body not preserved during redirect.\nExpected: %s\nReceived: %s", string(testData), string(receivedBody))
	}

	t.Log("✅ Send method correctly preserves request body during redirect")
}

func TestMain(m *testing.M) {
	fmt.Println("🧪 Running SDK Send Method Redirect Tests")
	fmt.Println("==========================================")
	result := m.Run()
	fmt.Println("==========================================")
	if result == 0 {
		fmt.Println("✅ All Send redirect tests passed!")
	} else {
		fmt.Println("❌ Some tests failed")
	}
	fmt.Println()
}
