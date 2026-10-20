package schema

import "fmt"

// HTTPStatusError reports an unsuccessful HTTP response.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("invalid server response: %d", e.StatusCode)
}
