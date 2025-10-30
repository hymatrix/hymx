package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RedirectErrorMiddleware handles redirect errors in a centralized way
func RedirectErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there's an error in the context
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if redirectErr, ok := err.(*nodeSchema.RedirectError); ok {
				// Handle redirect error
				handleRedirectError(c, redirectErr)
				return
			}
		}
	}
}

// handleRedirectError handles redirect errors by setting appropriate headers and response
func handleRedirectError(c *gin.Context, redirectErr *nodeSchema.RedirectError) {
	// Return 308 Permanent Redirect with Location header and nodes information
	if len(redirectErr.Nodes) > 0 {
		// Set Location header to the first available node URL for browser auto-redirect
		c.Header("Location", redirectErr.Nodes[0].URL)
	}
	// Also return nodes information in response body for client SDK usage
	c.JSON(http.StatusPermanentRedirect, redirectErr.Nodes)
}
