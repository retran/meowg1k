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

// Package configmocks provides mock implementations for config service.
package configmocks

import (
	"github.com/retran/meowg1k/internal/services/config"
)

// MockConfigService is a mock implementation of config.Service for testing.
type MockConfigService struct {
	Cfg *config.Config
}

// GetConfig implements config.Service.
func (m *MockConfigService) GetConfig() *config.Config {
	return m.Cfg
}
