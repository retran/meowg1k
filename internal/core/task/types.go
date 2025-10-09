package task

import "github.com/retran/meowg1k/internal/core/profile"

// ResolvedConfig represents a resolved task configuration.
type ResolvedConfig struct {
	Name         string
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}
