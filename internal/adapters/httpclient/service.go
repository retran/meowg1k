/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// Service provides a shared HTTP client for all HTTP-based gateways.
// Using a shared client is more efficient than creating new clients for each request,
// as http.Client manages connection pooling and reuse internally.
type Service struct {
	client *http.Client
}

// New creates a new HTTP client service with sensible defaults for LLM API calls.
// The returned client is safe for concurrent use and should be shared across
// all gateways that need HTTP connectivity.
func New() (*Service, error) {
	// Create HTTP client with configuration optimized for LLM API calls
	client := &http.Client{
		// Conservative timeout for long-running LLM API calls
		// Individual gateways can override this via context deadline if needed
		Timeout: 10 * time.Minute,

		// Custom transport with connection pooling optimizations
		Transport: &http.Transport{
			// Maximum number of idle connections across all hosts
			MaxIdleConns: 100,

			// Maximum number of idle connections per host
			// This prevents overwhelming any single API endpoint
			MaxIdleConnsPerHost: 10,

			// How long to keep idle connections before closing
			IdleConnTimeout: 90 * time.Second,

			// Enable HTTP/2 by default (will fall back to HTTP/1.1 if not supported)
			ForceAttemptHTTP2: true,

			// Timeouts for connection establishment
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,

			// TLS handshake timeout
			TLSHandshakeTimeout: 10 * time.Second,

			// Response header timeout
			ResponseHeaderTimeout: 30 * time.Second,

			// Expect-Continue timeout
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &Service{
		client: client,
	}, nil
}

// Get returns the underlying http.Client.
// The returned client is safe for concurrent use.
func (s *Service) Get() *http.Client {
	if s == nil {
		// Defensive programming: return a default client if service is nil
		// This should never happen in production but prevents panics
		return http.DefaultClient
	}
	return s.client
}

// GetWithTimeout returns a new http.Client with custom timeout settings.
// This is useful for operations that need different timeout characteristics
// than the default client (e.g., longer timeouts for embedding batch processing).
// The returned client shares the same connection pool as the base client.
func (s *Service) GetWithTimeout(timeout time.Duration) *http.Client {
	if s == nil {
		return http.DefaultClient
	}

	baseTransport := s.client.Transport.(*http.Transport)

	// Clone the transport with adjusted timeouts
	transport := &http.Transport{
		MaxIdleConns:          baseTransport.MaxIdleConns,
		MaxIdleConnsPerHost:   baseTransport.MaxIdleConnsPerHost,
		IdleConnTimeout:       baseTransport.IdleConnTimeout,
		ForceAttemptHTTP2:     baseTransport.ForceAttemptHTTP2,
		TLSHandshakeTimeout:   baseTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: baseTransport.ExpectContinueTimeout,

		// Use longer response header timeout to match the overall timeout
		// This is critical for batch operations that may take a long time
		ResponseHeaderTimeout: timeout,

		// Reuse the same dialer
		DialContext: baseTransport.DialContext,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// Close cleans up any resources held by the HTTP client.
// This is typically called during application shutdown.
func (s *Service) Close() error {
	if s == nil || s.client == nil {
		return nil
	}

	// Close idle connections
	if transport, ok := s.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	return nil
}

// Validate checks if the service is properly initialized.
func (s *Service) Validate() error {
	if s == nil {
		return fmt.Errorf("http client service is nil")
	}
	if s.client == nil {
		return fmt.Errorf("http client is nil")
	}
	return nil
}
