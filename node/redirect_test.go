package node

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/node/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
)

// TestRedirectResponse tests basic 308 redirect with multiple nodes
func TestRedirectResponse(t *testing.T) {
	// Create test nodes
	nodes := []registrySchema.Node{
		{
			AccId: "test-node-1",
			Name:  "Test Node 1",
			Role:  "main",
			Desc:  "Test redirect node",
			URL:   "http://node1.example.com:8080",
		},
		{
			AccId: "test-node-2",
			Name:  "Test Node 2",
			Role:  "follower",
			Desc:  "Test redirect node 2",
			URL:   "http://node2.example.com:8080",
		},
	}

	// Create redirect error
	redirectErr := schema.NewRedirectError(nodes)

	// Setup test server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/submit", func(c *gin.Context) {
		// Simulate redirect error handling
		if len(redirectErr.Nodes) > 0 {
			// Set Location header to the first available node URL for browser auto-redirect
			c.Header("Location", redirectErr.Nodes[0].URL)
		}
		// Also return nodes information in response body for client SDK usage
		c.JSON(http.StatusPermanentRedirect, redirectErr.Nodes)
	})

	// Test the redirect response
	req := httptest.NewRequest("POST", "/submit", strings.NewReader(`{"test": "data"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify results
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("Expected status code %d, got %d", http.StatusPermanentRedirect, w.Code)
	}
	if location := w.Header().Get("Location"); location != "http://node1.example.com:8080" {
		t.Errorf("Expected Location header 'http://node1.example.com:8080', got '%s'", location)
	}
	if contentType := w.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected Content-Type 'application/json; charset=utf-8', got '%s'", contentType)
	}
}

// TestEmptyNodesRedirect tests redirect with empty nodes list
func TestEmptyNodesRedirect(t *testing.T) {
	// Create redirect error with empty nodes
	redirectErr := schema.NewRedirectError([]registrySchema.Node{})

	// Setup test server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/submit", func(c *gin.Context) {
		// Simulate redirect error handling with empty nodes
		if len(redirectErr.Nodes) > 0 {
			c.Header("Location", redirectErr.Nodes[0].URL)
		}
		c.JSON(http.StatusPermanentRedirect, redirectErr.Nodes)
	})

	// Test the redirect response
	req := httptest.NewRequest("POST", "/submit", strings.NewReader(`{"test": "data"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify results
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("Expected status code %d, got %d", http.StatusPermanentRedirect, w.Code)
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("Expected empty Location header, got '%s'", location)
	}
	if body := w.Body.String(); body != "[]" {
		t.Errorf("Expected empty array '[]', got '%s'", body)
	}
}

// TestSingleNodeRedirect tests redirect with single node
func TestSingleNodeRedirect(t *testing.T) {
	// Create single test node
	nodes := []registrySchema.Node{
		{
			AccId: "single-node",
			Name:  "Single Node",
			Role:  "main",
			Desc:  "Single test node",
			URL:   "http://single.example.com:8080",
		},
	}

	// Create redirect error
	redirectErr := schema.NewRedirectError(nodes)

	// Setup test server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/submit", func(c *gin.Context) {
		if len(redirectErr.Nodes) > 0 {
			c.Header("Location", redirectErr.Nodes[0].URL)
		}
		c.JSON(http.StatusPermanentRedirect, redirectErr.Nodes)
	})

	// Test the redirect response
	req := httptest.NewRequest("POST", "/submit", strings.NewReader(`{"test": "data"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify results
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("Expected status code %d, got %d", http.StatusPermanentRedirect, w.Code)
	}
	if location := w.Header().Get("Location"); location != "http://single.example.com:8080" {
		t.Errorf("Expected Location header 'http://single.example.com:8080', got '%s'", location)
	}
}
