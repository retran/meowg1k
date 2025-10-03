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

// Package servicemocks provides re-exports of service mocks.
// This package is kept for backward compatibility.
// Use specific subpackages (configmocks, profilemocks, gatewaymocks, commandmocks) instead.
package servicemocks

import (
	"github.com/retran/meowg1k/internal/testutil/commandmocks"
	"github.com/retran/meowg1k/internal/testutil/configmocks"
	"github.com/retran/meowg1k/internal/testutil/gatewaymocks"
	"github.com/retran/meowg1k/internal/testutil/profilemocks"
)

// Re-export mocks for backward compatibility
type MockConfigService = configmocks.MockConfigService
type MockProfileService = profilemocks.MockProfileService
type MockGenerationGateway = gatewaymocks.MockGenerationGateway
type MockGatewayFactory = gatewaymocks.MockGatewayFactory
type MockCommandService = commandmocks.MockCommandService
