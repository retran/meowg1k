// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpclient provides a configured HTTP client with custom transport settings and timeouts.
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
// The returned client is safe for shared use and should be shared across
// all gateways that need HTTP connectivity.
func New() (*Service, error) {
	client := &http.Client{
		// Individual gateways can override this via context deadline if needed.
		Timeout: 10 * time.Minute,

		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &Service{
		client: client,
	}, nil
}

// Get returns the underlying http.Client.
// The returned client is safe for shared use.
func (s *Service) Get() *http.Client {
	if s == nil {
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

	baseTransport, ok := s.client.Transport.(*http.Transport)
	if !ok {
		return s.client
	}

	transport := &http.Transport{
		MaxIdleConns:          baseTransport.MaxIdleConns,
		MaxIdleConnsPerHost:   baseTransport.MaxIdleConnsPerHost,
		IdleConnTimeout:       baseTransport.IdleConnTimeout,
		ForceAttemptHTTP2:     baseTransport.ForceAttemptHTTP2,
		TLSHandshakeTimeout:   baseTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: baseTransport.ExpectContinueTimeout,
		// ResponseHeaderTimeout matches the overall timeout for batch operations.
		ResponseHeaderTimeout: timeout,
		DialContext:           baseTransport.DialContext,
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
