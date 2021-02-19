package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var NewLogger = func(name string) zerolog.Logger {
	w := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}
	return zerolog.New(w).With().Timestamp().Str(zerolog.CallerFieldName, name).Logger()
}
