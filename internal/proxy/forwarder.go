package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/yanghuajun/proxy/internal/trojan"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// forwardRequest forwards an HTTP request through a trojan client
func forwardRequest(client *trojan.Client, req *http.Request, w http.ResponseWriter) error {
	logger.Debug("Forwarding HTTP request to %s via %s", req.URL.String(), client.GetNodeName())

	// Create HTTP client that uses Trojan for dialing
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Use Trojan client to dial
				return client.Dial(ctx, network, addr)
			},
		},
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
