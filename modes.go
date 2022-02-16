package october

import (
	"os"
	"strings"
)

// Modes define October behavior across configurations
type Mode uint8

func (m Mode) String() string {
	switch m {
	case LOCAL:
		return "LOCAL"
	case DEV:
		return "DEV"
	case STAGE:
		return "STAGE"
	case PROD:
		return "PROD"
	}

	return "UNKNOWN"
}

const (
	LOCAL Mode = iota // Running locally
	DEV               // Running in development, the inference here is "development remotely"
	STAGE             // Running in staging, the inference here is "staging remotely"
	PROD              // Running in production
)

// Returns the mode from environment variable, as well as if it was found or not
func ModeFromEnv() (Mode, bool) {

	envVal := strings.TrimSpace(strings.ToUpper(os.Getenv(modeEnvVariable)))

	switch envVal {
	case "LOCAL":
		return LOCAL, true
	case "DEV":
		return DEV, true
	case "STAGE":
		return STAGE, true
	case "PROD":
		return PROD, true
	}

	return LOCAL, false
}
