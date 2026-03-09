// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport is a custom RoundTripper for testing non-Transport scenarios.
type mockTransport struct{}

func (m *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

func TestNew(t *testing.T) {
	service, err := New()
	require.NoError(t, err)
	require.NotNil(t, service)
	require.NotNil(t, service.client)
}

func TestGet(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	client := service.Get()
	require.NotNil(t, client)
	assert.IsType(t, &http.Client{}, client)
}

func TestGetWithNilService(t *testing.T) {
	var service *Service
	client := service.Get()
	require.NotNil(t, client)
	assert.Equal(t, http.DefaultClient, client)
}

func TestClientConfiguration(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	client := service.Get()
	require.NotNil(t, client)

	// Check timeout is set
	assert.Equal(t, 10*time.Minute, client.Timeout)

	// Check transport is configured
	require.NotNil(t, client.Transport)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	// Check connection pool settings
	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
	assert.True(t, transport.ForceAttemptHTTP2)
}

func TestClose(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	err = service.Close()
	assert.NoError(t, err)
}

func TestCloseWithNilService(t *testing.T) {
	var service *Service
	err := service.Close()
	assert.NoError(t, err)
}

func TestValidate(t *testing.T) {
	t.Run("Valid service", func(t *testing.T) {
		service, err := New()
		require.NoError(t, err)

		err = service.Validate()
		assert.NoError(t, err)
	})

	t.Run("Nil service", func(t *testing.T) {
		var service *Service
		err := service.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http client service is nil")
	})

	t.Run("Nil client", func(t *testing.T) {
		service := &Service{client: nil}
		err := service.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http client is nil")
	})
}

func TestClientReuse(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	// Get client multiple times
	client1 := service.Get()
	client2 := service.Get()

	// Should return the same instance
	assert.Same(t, client1, client2)
}

// TestGetWithTimeout tests creating clients with custom timeouts.
func TestGetWithTimeout(t *testing.T) {
	t.Run("valid service with custom timeout", func(t *testing.T) {
		service, err := New()
		require.NoError(t, err)

		customTimeout := 30 * time.Second
		client := service.GetWithTimeout(customTimeout)
		require.NotNil(t, client)

		// Check timeout is set correctly
		assert.Equal(t, customTimeout, client.Timeout)

		// Check transport settings are preserved
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, 100, transport.MaxIdleConns)
		assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
		assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
		assert.True(t, transport.ForceAttemptHTTP2)

		// Check response header timeout matches overall timeout
		assert.Equal(t, customTimeout, transport.ResponseHeaderTimeout)
	})

	t.Run("with nil service", func(t *testing.T) {
		var service *Service
		client := service.GetWithTimeout(5 * time.Second)
		require.NotNil(t, client)
		assert.Equal(t, http.DefaultClient, client)
	})

	t.Run("different timeout values", func(t *testing.T) {
		service, err := New()
		require.NoError(t, err)

		// Test various timeout values
		timeouts := []time.Duration{
			1 * time.Second,
			1 * time.Minute,
			30 * time.Minute,
		}

		for _, timeout := range timeouts {
			client := service.GetWithTimeout(timeout)
			require.NotNil(t, client)
			assert.Equal(t, timeout, client.Timeout)
		}
	})

	t.Run("clients with different timeouts are different instances", func(t *testing.T) {
		service, err := New()
		require.NoError(t, err)

		client1 := service.GetWithTimeout(1 * time.Minute)
		client2 := service.GetWithTimeout(2 * time.Minute)

		// Should be different instances
		assert.NotSame(t, client1, client2)
		assert.Equal(t, 1*time.Minute, client1.Timeout)
		assert.Equal(t, 2*time.Minute, client2.Timeout)
	})

	t.Run("with non-transport client", func(t *testing.T) {
		// Create service with non-Transport RoundTripper
		service := &Service{
			client: &http.Client{
				Transport: &mockTransport{},
			},
		}

		// Should return base client when transport is not *http.Transport
		client := service.GetWithTimeout(5 * time.Second)
		require.NotNil(t, client)
		assert.Same(t, service.client, client)
	})
}

// TestCloseIdleConnections tests that Close properly closes idle connections.
func TestCloseIdleConnections(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	// Close should succeed
	err = service.Close()
	assert.NoError(t, err)

	// Should be safe to close multiple times
	err = service.Close()
	assert.NoError(t, err)
}

// TestCloseWithNilClient tests Close with nil internal client.
func TestCloseWithNilClient(t *testing.T) {
	service := &Service{client: nil}
	err := service.Close()
	assert.NoError(t, err)
}

// TestTransportSettings tests detailed transport configuration.
func TestTransportSettings(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	client := service.Get()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	// Test all timeout settings
	assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, 30*time.Second, transport.ResponseHeaderTimeout)
	assert.Equal(t, 1*time.Second, transport.ExpectContinueTimeout)

	// Test dialer settings
	require.NotNil(t, transport.DialContext)
}

// TestGetWithTimeoutPreservesDialer tests that custom timeout clients preserve the dialer.
func TestGetWithTimeoutPreservesDialer(t *testing.T) {
	service, err := New()
	require.NoError(t, err)

	baseClient := service.Get()
	baseTransport := baseClient.Transport.(*http.Transport)

	customClient := service.GetWithTimeout(5 * time.Minute)
	customTransport := customClient.Transport.(*http.Transport)

	// Dialer should be preserved (both should be non-nil)
	assert.NotNil(t, baseTransport.DialContext)
	assert.NotNil(t, customTransport.DialContext)

	// But response header timeout should be updated
	assert.NotEqual(t, baseTransport.ResponseHeaderTimeout, customTransport.ResponseHeaderTimeout)
	assert.Equal(t, 5*time.Minute, customTransport.ResponseHeaderTimeout)
}
