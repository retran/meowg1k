package profiles

import (
	"testing"

	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/providers"
	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	// Create services
	registryService := providers.NewService()
	configService := configservice.NewService(nil)
	profileService := NewService(registryService, configService)

	// Test that the service can be created
	assert.NotNil(t, profileService, "Profile service should be created successfully")

	// Test that the service implements the expected interface
	var _ Service = profileService
}
