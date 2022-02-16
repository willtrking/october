package october

import (
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func initService(mode Mode, fromEnv bool) (*OctoberServer, error) {

	loggerErr := ConfigureZap(mode)
	if loggerErr != nil {
		return nil, loggerErr
	}

	zap.L().Named("OCTOBER").Info("October configured zap")

	zap.L().Named("OCTOBER").Info("Initializing October...")
	if fromEnv {
		zap.L().Named("OCTOBER").Info("October Mode (from env): " + mode.String())
	} else {
		zap.L().Named("OCTOBER").Info("October Mode: " + mode.String())
	}

	port := 0
	envPort := strings.TrimSpace(os.Getenv(portEnvVariable))
	if envPort != "" {
		var err error
		port, err = strconv.Atoi(envPort)
		if err != nil {
			return nil, err
		}
	}

	server := NewOctoberServer(mode, port)

	return server, nil
}

func ConfigureZap(mode Mode) error {
	logger, loggerErr := NewZapLogger(mode)
	if loggerErr != nil {
		return loggerErr
	}

	zap.ReplaceGlobals(logger)

	return nil
}

func MustConfigureZap(mode Mode) {
	err := ConfigureZap(mode)
	if err != nil {
		panic(err)
	}
}

func ConfigureZapFromEnv() error {
	mode, _ := ModeFromEnv()
	return ConfigureZap(mode)
}

func MustConfigureZapFromEnv() {
	err := ConfigureZapFromEnv()
	if err != nil {
		panic(err)
	}
}

// InitService configures the following:
// - Zap
func InitService(mode Mode) (*OctoberServer, error) {
	return initService(mode, false)
}

func InitServiceFromEnv() (*OctoberServer, error) {
	mode, found := ModeFromEnv()
	return initService(mode, found)
}

func MustInitService(mode Mode) *OctoberServer {
	server, err := InitService(mode)
	if err != nil {
		panic(err)
	}

	return server
}

func MustInitServiceFromEnv() *OctoberServer {
	server, err := InitServiceFromEnv()
	if err != nil {
		panic(err)
	}
	return server
}

func CleanupService() {
	zap.L().Sync()

}
