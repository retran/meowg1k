// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
