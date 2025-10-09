package output

// Destination represents where the output should be sent.
type Destination string

const (
	// Stdout sends output to standard output.
	Stdout Destination = "stdout"
	// Stderr sends output to standard error.
	Stderr Destination = "stderr"
	// Discard discards all output.
	Discard Destination = "discard"
)
