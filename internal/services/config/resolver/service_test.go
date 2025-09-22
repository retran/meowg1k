package resolver

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/gateway"
	llmRegistry "github.com/retran/meowg1k/internal/services/llm/registry"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock services for testing
type mockRegistryService struct {
	mock.Mock
}

func newMockRegistryService() *mockRegistryService {
	return &mockRegistryService{}
}

func (m *mockRegistryService) RegisterProvider(name string, definition config.ProviderDefinition) error {
	args := m.Called(name, definition)
	return args.Error(0)
}

func (m *mockRegistryService) GetProvider(name string) (config.ProviderDefinition, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return config.ProviderDefinition{}, args.Error(1)
	}
	return args.Get(0).(config.ProviderDefinition), args.Error(1)
}

func (m *mockRegistryService) ListProviders() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *mockRegistryService) HasProvider(name string) bool {
	args := m.Called(name)
	return args.Bool(0)
}

func (m *mockRegistryService) GetDefaultProfile(providerType gateway.Provider) config.Profile {
	args := m.Called(providerType)
	return args.Get(0).(config.Profile)
}

type mockValidatorService struct {
	mock.Mock
}

func newMockValidatorService() *mockValidatorService {
	return &mockValidatorService{}
}

func (m *mockValidatorService) ValidateResolvedProfile(profile *config.ResolvedProfile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *mockValidatorService) ValidateConfig(cfg *config.Config) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *mockValidatorService) ValidateProfile(profile *config.Profile, profileName string) error {
	args := m.Called(profile, profileName)
	return args.Error(0)
}

type mockCommandService struct {
	mock.Mock
}

func newMockCommandService() *mockCommandService {
	return &mockCommandService{}
}

func (m *mockCommandService) GetCommand() *cobra.Command {
	args := m.Called()
	return args.Get(0).(*cobra.Command)
}

type mockManagerService struct {
	mock.Mock
}

func newMockManagerService() *mockManagerService {
	return &mockManagerService{}
}

func (m *mockManagerService) GetConfig() *config.Config {
	args := m.Called()
	return args.Get(0).(*config.Config)
}

func (m *mockManagerService) GetConfigPath() string {
	args := m.Called()
	return args.String(0)
}

func TestNewService(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	assert.NotNil(t, service)
	// Verify it implements the Service interface
	var _ Service = service
}

func TestResolveProfile_Success(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Setup mock expectations
	providerDef := config.ProviderDefinition{
		Type:            gateway.OpenAI,
		Name:            "OpenAI",
		DefaultModel:    "gpt-4o",
		DefaultBaseURL:  "https://api.openai.com/v1",
		DefaultEnvVar:   "OPENAI_API_KEY",
		RequiresAPIKey:  true,
		RequiresBaseURL: false,
		TokenizerType:   llmRegistry.TokenizerCL100K,
		MaxInputTokens:  128000,
		MaxOutputTokens: 32768,
		DefaultTimeout:  5 * time.Minute,
	}

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"test-profile": {
				Provider:       "openai",
				Model:          "gpt-4o",
				MaxInputTokens: 100000,
			},
		},
	}

	mockRegistry.On("GetProvider", "openai").Return(providerDef, nil)
	mockManager.On("GetConfig").Return(cfg)
	mockValidator.On("ValidateResolvedProfile", mock.AnythingOfType("*config.ResolvedProfile")).Return(nil)

	// Set up environment variable
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Execute
	result, err := service.ResolveProfile("test-profile")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, gateway.OpenAI, result.Provider)
	assert.Equal(t, "gpt-4o", result.Model)
	assert.Equal(t, "test-key", result.APIKey)
	assert.Equal(t, 100000, result.MaxInputTokens)

	mockRegistry.AssertExpectations(t)
	mockManager.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
}

func TestResolveProfile_WithDefaults(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Setup mock expectations
	providerDef := config.ProviderDefinition{
		Type:            gateway.OpenAI,
		Name:            "OpenAI",
		DefaultModel:    "gpt-3.5-turbo",
		DefaultBaseURL:  "https://api.openai.com/v1",
		DefaultEnvVar:   "OPENAI_API_KEY",
		RequiresAPIKey:  true,
		RequiresBaseURL: false,
		TokenizerType:   llmRegistry.TokenizerCL100K,
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		DefaultTimeout:  5 * time.Minute,
	}

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"minimal-profile": {
				Provider: "openai",
				// All other fields use defaults
			},
		},
	}

	mockRegistry.On("GetProvider", "openai").Return(providerDef, nil)
	mockManager.On("GetConfig").Return(cfg)
	mockValidator.On("ValidateResolvedProfile", mock.AnythingOfType("*config.ResolvedProfile")).Return(nil)

	// Set up environment variable
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Execute
	result, err := service.ResolveProfile("minimal-profile")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, gateway.OpenAI, result.Provider)
	assert.Equal(t, "gpt-3.5-turbo", result.Model)               // From provider default
	assert.Equal(t, 128000, result.MaxInputTokens)               // From provider default
	assert.Equal(t, 4096, result.MaxOutputTokens)                // From provider default
	assert.Equal(t, 5*time.Minute, result.Timeout)               // From provider default
	assert.Equal(t, "https://api.openai.com/v1", result.BaseURL) // From provider default
	assert.Equal(t, "test-key", result.APIKey)
	assert.Equal(t, llmRegistry.TokenizerCL100K, result.TokenizerType)

	mockRegistry.AssertExpectations(t)
	mockManager.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
}

func TestResolveProfile_ProfileNotFound(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{}, // Empty profiles
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := service.ResolveProfile("nonexistent-profile")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "profile 'nonexistent-profile' not found")

	mockManager.AssertExpectations(t)
}

func TestResolveProfile_NoProfilesDefined(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{} // No profiles defined

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := service.ResolveProfile("any-profile")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no profiles defined")

	mockManager.AssertExpectations(t)
}

func TestResolveProfile_UnknownProvider(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"test-profile": {
				Provider: "unknown-provider",
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)
	mockRegistry.On("GetProvider", "unknown-provider").Return(config.ProviderDefinition{}, errors.New("unknown provider"))

	// Execute
	result, err := service.ResolveProfile("test-profile")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown provider 'unknown-provider'")

	mockRegistry.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveProfile_ValidationFailure(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Setup mock expectations
	providerDef := config.ProviderDefinition{
		Type:           gateway.OpenAI,
		DefaultEnvVar:  "OPENAI_API_KEY",
		RequiresAPIKey: true,
	}

	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"test-profile": {
				Provider: "openai",
			},
		},
	}

	mockRegistry.On("GetProvider", "openai").Return(providerDef, nil)
	mockManager.On("GetConfig").Return(cfg)
	mockValidator.On("ValidateResolvedProfile", mock.AnythingOfType("*config.ResolvedProfile")).Return(errors.New("validation failed"))

	// Execute
	result, err := service.ResolveProfile("test-profile")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "profile validation failed")

	mockRegistry.AssertExpectations(t)
	mockManager.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
}

func TestResolvePrompt_Success(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					UserPrompt: "Test user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := service.ResolvePrompt("test-task")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Test user prompt", result)

	mockManager.AssertExpectations(t)
}

func TestResolvePrompt_PromptNotFound(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{}, // Empty tasks
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := service.ResolvePrompt("nonexistent-prompt")

	// Assert
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "prompt 'nonexistent-prompt' not found")

	mockManager.AssertExpectations(t)
}

func TestResolvePrompt_NoTasksConfigured(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: nil,
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := service.ResolvePrompt("any-prompt")

	// Assert
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "prompt 'any-prompt' not found")

	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockManager := newMockManagerService()
	mockCommand := newMockCommandService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create command with task flag
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "test-task", "")
	cmd.Flags().String("user-prompt", "custom user prompt", "")
	mockCommand.On("GetCommand").Return(cmd)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					Profile:      "test-profile",
					SystemPrompt: "Test system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profileName)
	assert.Equal(t, "Test system prompt", systemPrompt)
	assert.Equal(t, "custom user prompt", userPrompt) // Command-line override

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_CommandLineUserPromptOverride(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with task flag and user-prompt override
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "test-task", "")
	cmd.Flags().String("user-prompt", "Command line user prompt", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					Profile:      "task-profile",
					SystemPrompt: "Task system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	assert.NoError(t, err)
	assert.Equal(t, "task-profile", profileName)
	assert.Equal(t, "Task system prompt", systemPrompt)
	assert.Equal(t, "Command line user prompt", userPrompt) // Should be overridden

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_NoTaskProfile_UsesDefault(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with task flag set
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "test-task", "")
	cmd.Flags().String("user-prompt", "", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					// No profile specified, should use default
					SystemPrompt: "Task system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	assert.NoError(t, err)
	assert.Equal(t, "default-profile", profileName) // Should use default
	assert.Equal(t, "Task system prompt", systemPrompt)
	assert.Equal(t, "Task user prompt", userPrompt)

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_TaskNotFound(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with task flag set
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "nonexistent-task", "")
	cmd.Flags().String("user-prompt", "", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	_, _, _, err := service.ResolveTaskConfiguration()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task 'nonexistent-task' not found")

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_NoTasksConfigured(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with task flag set
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "any-task", "")
	cmd.Flags().String("user-prompt", "", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: nil,
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	_, _, _, err := service.ResolveTaskConfiguration()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tasks configured")

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_DefaultConfiguration(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with no task flag, but with user-prompt
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "", "")
	cmd.Flags().String("user-prompt", "Command line user prompt", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	assert.NoError(t, err)
	assert.Equal(t, "default-profile", profileName)
	assert.Equal(t, "Default system prompt", systemPrompt)
	assert.Equal(t, "Command line user prompt", userPrompt)

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_DefaultConfiguration_MissingUserPrompt(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with no task flag and no user-prompt
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "", "")
	cmd.Flags().String("user-prompt", "", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	_, _, _, err := service.ResolveTaskConfiguration()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user prompt is required")

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestResolveTaskConfiguration_DefaultConfiguration_NoDefaultProfile(t *testing.T) {
	mockRegistry := newMockRegistryService()
	mockValidator := newMockValidatorService()
	mockCommand := newMockCommandService()
	mockManager := newMockManagerService()

	service := NewService(mockRegistry, mockValidator, mockCommand, mockManager)

	// Create a command with no task flag, but with user-prompt
	cmd := &cobra.Command{}
	cmd.Flags().String("task", "", "")
	cmd.Flags().String("user-prompt", "Command line user prompt", "")

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				// No profile specified
				SystemPrompt: "Default system prompt",
			},
		},
	}

	mockCommand.On("GetCommand").Return(cmd)
	mockManager.On("GetConfig").Return(cfg)

	_, _, _, err := service.ResolveTaskConfiguration()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default profile configured")

	mockCommand.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}
