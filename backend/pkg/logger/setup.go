package logger

import (
	"os"

	"go.uber.org/zap"
)

// New builds a production (or development) zap logger tagged with the service name.
// Set LOG_LEVEL=debug for development encoding.
func New(service string) *zap.Logger {
	var (
		log *zap.Logger
		err error
	)
	if os.Getenv("LOG_LEVEL") == "debug" {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}
	if err != nil {
		log = zap.NewNop()
	}
	return log.With(zap.String("service", service))
}
