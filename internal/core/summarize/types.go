package summarize

import (
	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/profile"
)

// ResolvedConfig holds the resolved summarization configuration for a specific file.
type ResolvedConfig struct {
	Profile             *profile.ResolvedProfile
	Strategy            *config.Strategy
	SystemPrompt        string
	Skip                bool
	IncludeOriginalFile bool
	IncludeChangedFile  bool
}
