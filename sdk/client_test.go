package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	serverSchema "github.com/hymatrix/hymx/server/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Send Method Redirect Tests
// ========================================

// TestSendRedirectHandling tests the Send method's 308 redirect functionality
func TestSendRedirectHandling(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and content type
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

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
	require.NoError(t, err)

	// Verify redirected URL is set
	assert.NotEmpty(t, redirectedURL)

	// Verify response is not nil
	require.NotNil(t, response)

	// Verify response
	assert.Equal(t, "test-message-id", response.Id)
	assert.Equal(t, "success", response.Message)

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
	require.Error(t, err)

	// The response should be nil since we can't parse a successful response from 308
	assert.Nil(t, response)

	// Redirected URL should be empty when all nodes fail
	assert.Empty(t, redirectedURL)

	t.Log("✅ Send method correctly handled failed alternative nodes")
}

// TestSendWithoutRedirect tests Send method with normal successful response
func TestSendWithoutRedirect(t *testing.T) {
	// Create a mock successful server
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody := `{"test": "data"}`
		assert.Equal(t, expectedBody, string(body))

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
	require.NoError(t, err)

	// Redirected URL should be empty when no redirect occurs
	assert.Empty(t, redirectedURL)

	// Verify response is not nil
	require.NotNil(t, response)

	// Verify response
	assert.Equal(t, "direct-message-id", response.Id)
	assert.Equal(t, "direct-success", response.Message)

	t.Log("✅ Send method works correctly without redirect")
}

// TestSendRedirectPreservesRequestBody tests that request body is preserved during redirect
func TestSendRedirectPreservesRequestBody(t *testing.T) {
	var receivedBody []byte

	// Create a mock successful server that captures the request body
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
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
	require.NoError(t, err)

	// Verify redirected URL is set
	assert.NotEmpty(t, redirectedURL)

	// Verify the request body was preserved
	assert.Equal(t, testData, receivedBody)

	t.Log("✅ Send method correctly preserves request body during redirect")
}

// ========================================
// GetNodes Method Tests
// ========================================

// TestGetNodesSuccess tests successful retrieval of nodes map
func TestGetNodesSuccess(t *testing.T) {
	// Prepare mock nodes
	nodes := map[string]registrySchema.Node{
		"acc1": {AccId: "acc1", Name: "node-1", Role: "main", Desc: "desc-1", URL: "http://127.0.0.1:8080"},
		"acc2": {AccId: "acc2", Name: "node-2", Role: "follower", Desc: "desc-2", URL: "http://127.0.0.1:8081"},
	}

	// Create mock server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodes", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(nodes)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodes()
	require.NoError(t, err)

	require.Len(t, got, 2)
	assert.Equal(t, "http://127.0.0.1:8080", got["acc1"].URL)
	assert.Equal(t, "follower", got["acc2"].Role)

	t.Log("✅ GetNodes method returns nodes map correctly")
}

// TestGetNodesErrorStatus tests non-2xx status handling
func TestGetNodesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodes()
	require.Error(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNodes method handles non-2xx status correctly")
}

// TestGetNodesNullResponse tests decoding of null response body
func TestGetNodesNullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodes()
	require.NoError(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNodes method decodes null response to nil map")
}

// ========================================
// GetNode Method Tests
// ========================================

// TestGetNodeSuccess verifies /node/:accid returns a single node
func TestGetNodeSuccess(t *testing.T) {
	want := registrySchema.Node{AccId: "acc123", Name: "node-main", Role: "main", Desc: "desc", URL: "http://127.0.0.1:8080"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/node/acc123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNode("acc123")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.URL, got.URL)
	assert.Equal(t, want.Role, got.Role)

	t.Log("✅ GetNode method returns single node correctly")
}

// TestGetNodeErrorStatus verifies non-2xx status handling
func TestGetNodeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNode("acc500")
	require.Error(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNode method handles non-2xx status correctly")
}

// TestGetNodeNullResponse verifies null body yields nil pointer
func TestGetNodeNullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNode("accnull")
	require.NoError(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNode method decodes null response to nil pointer")
}

// ========================================
// GetNodesByProcess Method Tests
// ========================================

// TestGetNodesByProcessSuccess tests retrieval of nodes by process
func TestGetNodesByProcessSuccess(t *testing.T) {
	want := []registrySchema.Node{
		{AccId: "acc1", Name: "node-1", Role: "main", Desc: "d1", URL: "http://127.0.0.1:8080"},
		{AccId: "acc2", Name: "node-2", Role: "candidate", Desc: "d2", URL: "http://127.0.0.1:8081"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodesByProcess/p123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodesByProcess("p123")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "acc1", got[0].AccId)
	assert.Equal(t, "candidate", got[1].Role)

	t.Log("✅ GetNodesByProcess returns node list correctly")
}

// TestGetNodesByProcessErrorStatus tests non-2xx status
func TestGetNodesByProcessErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodesByProcess("p500")
	require.Error(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNodesByProcess handles non-2xx status correctly")
}

// TestGetNodesByProcessNullResponse tests decoding of null to nil slice
func TestGetNodesByProcessNullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetNodesByProcess("pnull")
	require.NoError(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetNodesByProcess decodes null to nil slice")
}

// ========================================
// GetProcesses Method Tests
// ========================================

// TestGetProcessesSuccess tests retrieval of processes by accid
func TestGetProcessesSuccess(t *testing.T) {
	want := []string{"p1", "p2", "p3"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/processes/accX", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetProcesses("accX")
	require.NoError(t, err)
	assert.Equal(t, want, got)

	t.Log("✅ GetProcesses returns process list correctly")
}

// TestGetProcessesErrorStatus tests non-2xx status handling
func TestGetProcessesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetProcesses("acc500")
	require.Error(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetProcesses handles non-2xx status correctly")
}

// TestGetProcessesNullResponse tests null body decoding to nil slice
func TestGetProcessesNullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetProcesses("accnull")
	require.NoError(t, err)
	assert.Nil(t, got)

	t.Log("✅ GetProcesses decodes null to nil slice")
}

// ========================================
// GetCache Method Tests
// ========================================

// TestGetCacheSuccess tests successful retrieval of cache value
func TestGetCacheSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cache/p123/k456", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode("cached-value")
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetCache("p123", "k456")
	require.NoError(t, err)
	assert.Equal(t, "cached-value", got)

	t.Log("✅ GetCache returns string value correctly")
}

// TestGetCacheErrorStatus tests non-2xx status handling
func TestGetCacheErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetCache("p500", "k500")
	require.Error(t, err)
	assert.Empty(t, got)

	t.Log("✅ GetCache handles non-2xx status correctly")
}

// TestGetCacheEmptyString tests decoding of JSON empty string
func TestGetCacheEmptyString(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cache/p123/k456", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("\"\"")) // JSON empty string ""
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetCache("p123", "k456")
	require.NoError(t, err)
	assert.Empty(t, got)

	t.Log("✅ GetCache decodes empty string correctly")
}

// ========================================
// TrySend Method Tests
// ========================================

// TestTrySendSuccess tests successful POST /trysend
func TestTrySendSuccess(t *testing.T) {
	var received serverSchema.TrySendRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/trysend", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// Read and decode body
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		require.Equal(t, "p123", received.Pid)
		require.Equal(t, "t456", received.Target)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	require.NoError(t, client.TrySend("p123", "t456"))

	t.Log("✅ TrySend posts JSON and returns on 2xx")
}

// TestTrySendErrorStatus tests non-2xx status handling
func TestTrySendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	require.Error(t, client.TrySend("p500", "t500"))

	t.Log("✅ TrySend handles non-2xx status correctly")
}

// ========================================
// GetResult Method Redirect Tests
// ========================================

// TestGetResultRedirectHandling tests the GetResult method's 308 redirect functionality
func TestGetResultRedirectHandling(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method
		assert.Equal(t, "GET", r.Method)

		// Verify the URL path contains the expected process ID and message ID
		expectedPath := "/result/test-process-id/test-message-id"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Return a successful result response
		result := vmmSchema.VmmResult{
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
	require.NoError(t, err)

	// Verify result
	assert.Equal(t, "test-item-id", result.ItemId)
	assert.Equal(t, "test-process-id", result.FromProcess)
	assert.Equal(t, "test-output", result.Output)

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
	require.Error(t, err)

	t.Logf("✅ GetResult correctly failed when all alternative nodes failed: %v", err)
}

// TestGetResultWithoutRedirect tests GetResult method with normal successful response
func TestGetResultWithoutRedirect(t *testing.T) {
	// Create a mock successful server
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)

		// Verify URL path
		expectedPath := "/result/direct-process-id/direct-message-id"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Return successful result response
		result := vmmSchema.VmmResult{
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
	require.NoError(t, err)

	// Verify result
	assert.Equal(t, "direct-item-id", result.ItemId)
	assert.Equal(t, "direct-process-id", result.FromProcess)
	assert.Equal(t, "direct-output", result.Output)

	t.Log("✅ GetResult method works correctly without redirect")
}

// TestGetResultRedirectPreservesURLPath tests that URL path is preserved during redirect
func TestGetResultRedirectPreservesURLPath(t *testing.T) {
	// Create a mock successful server (alternative node)
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the URL path is preserved
		expectedPath := "/result/path-process-id/path-message-id"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Return successful result response
		result := vmmSchema.VmmResult{
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
	require.NoError(t, err)

	// Verify result
	assert.Equal(t, "path-test-id", result.ItemId)

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
		result := vmmSchema.VmmResult{
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
	require.NoError(t, err)

	// Verify result
	assert.Equal(t, "multi-node-item-id", result.ItemId)
	assert.Equal(t, "multi-node-process-id", result.FromProcess)

	t.Log("✅ GetResult method successfully handled redirect with multiple nodes")
}

// ========================================
// Test Main Function
// ========================================}

// ========================================
// GetResults Method Tests
// ========================================

// TestGetResultsSuccess tests the GetResults method with successful response
func TestGetResultsSuccess(t *testing.T) {
	// Create mock server that returns ResponseResults
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "GET", r.Method)
		expectedPath := "/results/test-process-id"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Verify query parameters
		assert.Equal(t, "DESC", r.URL.Query().Get("sort"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		// Create mock response data
		mockResults := serverSchema.ResponseResults{
			Edges: []serverSchema.ResultsEdge{
				{
					Cursor: "eyJ0aW1lc3RhbXAiOjE2MzQ1Njc4OTAsIm9yZGluYXRlIjoxLCJjcm9uIjoiMS0xMC1taW51dGVzIiwic29ydCI6IkFTQyJ9",
					Node: vmmSchema.VmmResult{
						Nonce:       "1",
						Timestamp:   "1634567890",
						ItemId:      "test-item-1",
						FromProcess: "test-process-id",
						PushedFor:   "test-item-1",
						Messages:    []*vmmSchema.ResMessage{},
						Spawns:      []*vmmSchema.ResSpawn{},
						Assignments: []interface{}{},
						Output:      map[string]interface{}{"result": "success"},
						Data:        "test-data-1",
						Error:       "",
					},
				},
				{
					Cursor: "eyJ0aW1lc3RhbXAiOjE2MzQ1Njc4OTEsIm9yZGluYXRlIjoyLCJjcm9uIjoiMS0xMC1taW51dGVzIiwic29ydCI6IkFTQyJ9",
					Node: vmmSchema.VmmResult{
						Nonce:       "2",
						Timestamp:   "1634567891",
						ItemId:      "test-item-2",
						FromProcess: "test-process-id",
						PushedFor:   "test-item-2",
						Messages:    []*vmmSchema.ResMessage{},
						Spawns:      []*vmmSchema.ResSpawn{},
						Assignments: []interface{}{},
						Output:      map[string]interface{}{"result": "success"},
						Data:        "test-data-2",
						Error:       "",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResults)
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Call GetResults
	results, err := client.GetResults("test-process-id", 10)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, results.Edges, 2)

	// Verify first result
	firstEdge := results.Edges[0]
	assert.Equal(t, "1", firstEdge.Node.Nonce)
	assert.Equal(t, "test-item-1", firstEdge.Node.ItemId)
	assert.NotEmpty(t, firstEdge.Cursor)

	// Verify second result
	secondEdge := results.Edges[1]
	assert.Equal(t, "2", secondEdge.Node.Nonce)
	assert.Equal(t, "test-item-2", secondEdge.Node.ItemId)

	t.Log("✅ GetResults method works correctly with successful response")
}

// TestGetResultsEmptyResponse tests the GetResults method with empty results
func TestGetResultsEmptyResponse(t *testing.T) {
	// Create mock server that returns empty ResponseResults
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockResults := serverSchema.ResponseResults{
			Edges: []serverSchema.ResultsEdge{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResults)
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Call GetResults
	results, err := client.GetResults("empty-process-id", 5)
	require.NoError(t, err)

	// Verify empty results
	assert.Len(t, results.Edges, 0)

	t.Log("✅ GetResults method handles empty response correctly")
}

// TestGetResultsServerError tests the GetResults method with server error
func TestGetResultsServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Call GetResults
	_, err := client.GetResults("error-process-id", 5)
	require.Error(t, err)

	// Verify error message contains status code
	expectedError := "invalid server response: 500"
	assert.EqualError(t, err, expectedError)

	t.Log("✅ GetResults method handles server error correctly")
}

// TestGetResultsInvalidJSON tests the GetResults method with invalid JSON response
func TestGetResultsInvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Call GetResults
	_, err := client.GetResults("invalid-json-process-id", 5)
	require.Error(t, err)

	t.Log("✅ GetResults method handles invalid JSON correctly")
}

// TestGetResultsNetworkError tests the GetResults method with network error
func TestGetResultsNetworkError(t *testing.T) {
	// Create client with invalid URL
	client := NewClient("http://invalid-url-that-does-not-exist:9999")

	// Call GetResults
	_, err := client.GetResults("network-error-process-id", 5)
	require.Error(t, err)

	t.Log("✅ GetResults method handles network error correctly")
}

// TestGetResultsURLBuilding tests the GetResults method URL building with different parameters
func TestGetResultsURLBuilding(t *testing.T) {
	var capturedURL string

	// Create mock server that captures the request URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()

		mockResults := serverSchema.ResponseResults{
			Edges: []serverSchema.ResultsEdge{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResults)
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Test with different parameters
	testCases := []struct {
		pid          string
		limit        int64
		expectedPath string
	}{
		{"process-123", 5, "/results/process-123?sort=DESC&limit=5"},
		{"another-process", 20, "/results/another-process?sort=DESC&limit=20"},
		{"special-chars-process", 1, "/results/special-chars-process?sort=DESC&limit=1"},
	}

	for _, tc := range testCases {
		// Call GetResults
		_, err := client.GetResults(tc.pid, tc.limit)
		require.NoErrorf(t, err, "GetResults failed for pid %s", tc.pid)

		// Verify URL
		assert.Equal(t, tc.expectedPath, capturedURL)
	}

	t.Log("✅ GetResults method builds URLs correctly")
}

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
