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

// Package profilemocks provides mock implementations for profile service.
package profilemocks

import (
	"github.com/retran/meowg1k/internal/services/profile"
)

// MockProfileService is a mock implementation of profile.Service for testing.
type MockProfileService struct {
	Profile *profile.ResolvedProfile
	Err     error
}

// Get implements profile.Service.
func (m *MockProfileService) Get(p profile.Profile) (*profile.ResolvedProfile, error) {
	return m.Profile, m.Err
}
