package proxy

import (
	"fmt"
	"io"
	"net/http"

	"github.com/yanghuajun/proxy/internal/trojan"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// forwardRequest forwards an HTTP request through a trojan client
func forwardRequest(client *trojan.Client, req *http.Request, w http.ResponseWriter) error {
	// For MVP, we'll use a simplified approach
	// In full implementation, this would establish a connection through trojan
	// and send the HTTP request through that connection

	// Create HTTP client that uses trojan transport
	// For now, using default transport as placeholder
	// TODO: Integrate with trojan-go library for actual tunneling
	
	logger.Debug("Forwarding HTTP request to %s via %s", req.URL.String(), client.GetNodeName())

	// Send request (simplified - would go through trojan tunnel in full implementation)
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	return err
}
