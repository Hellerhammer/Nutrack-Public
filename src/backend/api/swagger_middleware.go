package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CustomSwaggerHandler creates a custom handler that wraps the swagger handler
// and modifies the response to replace the host with the one from the environment
func CustomSwaggerHandler(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only process doc.json requests
		if !strings.HasSuffix(c.Request.URL.Path, "/doc.json") {
			handler(c)
			return
		}

		// Get the host from environment
		host := os.Getenv("HOST_URL")
		if host == "" {
			host = "localhost"
		}
		if !strings.Contains(host, ":") {
			host += ":8080"
		}

		// Create a response writer that captures the response
		w := &responseRewriter{
			ResponseWriter: c.Writer,
			body:           []byte{},
			host:           host,
		}

		// Replace the writer and process the request
		c.Writer = w
		handler(c)

		// If we captured any response, process and send it
		if len(w.body) > 0 {
			content := string(w.body)
			content = strings.ReplaceAll(content, "localhost:8080", w.host)

			// Clear any existing headers that might have been set
			for k := range w.Header() {
				w.Header().Del(k)
			}

			// Set content type and length
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))

			// Write the response
			w.ResponseWriter.WriteHeader(http.StatusOK)
			w.ResponseWriter.Write([]byte(content))
		}
	}
}

type responseRewriter struct {
	gin.ResponseWriter
	body []byte
	host string
}

func (w *responseRewriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *responseRewriter) WriteHeader(statusCode int) {
	// Don't write the header yet
}

func (w *responseRewriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responseRewriter) WriteHeaderNow() {
	// Don't write the header yet
}

func (w *responseRewriter) CloseNotify() <-chan bool {
	if closeNotifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return closeNotifier.CloseNotify()
	}
	// Fallback for when CloseNotifier is not available
	return make(<-chan bool)
}

func (w *responseRewriter) Flush() {
	// Implement if needed for streaming responses
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
