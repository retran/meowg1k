package commit

import "github.com/retran/meowg1k/internal/core/profile"

// ResolvedConfig represents the resolved configuration for generating a commit message.
type ResolvedConfig struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
}
