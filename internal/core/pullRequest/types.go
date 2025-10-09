package pullRequest

import "github.com/retran/meowg1k/internal/core/profile"

// ResolvedConfig represents the resolved configuration for generating a PR description.
type ResolvedConfig struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
}
