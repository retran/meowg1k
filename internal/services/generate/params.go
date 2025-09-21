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

package generate

import (
	"github.com/retran/meowg1k/internal/config"
)

// Params holds all the resolved parameters for a generation request.
type Params struct {
	Profile      *config.ResolvedProfile // Resolved configuration profile for the LLM
	SystemPrompt string                  // System-level instruction for the LLM
	UserPrompt   string                  // User's main prompt, potentially combined with stdin
}
