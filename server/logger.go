package main

import (
	"os"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/rs/zerolog"
)

var NewLogger = func(name string) zerolog.Logger {
	w := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}

	shouldDebug := false

	if debug := os.Getenv("DEBUG"); len(debug) > 0 {
		for _, part := range strings.Split(debug, ",") {
			part := strings.TrimSpace(part)
			if len(part) == 0 {
				continue
			}

			shouldMatch := true
			if part[0] == '-' {
				shouldMatch = false
				part = part[1:]
			}
			if g := glob.MustCompile(part); g.Match(name) {
				shouldDebug = shouldMatch
			}
		}
	} else {
		shouldDebug = true
	}

	level := zerolog.InfoLevel

	if shouldDebug {
		level = zerolog.DebugLevel
	}

	return zerolog.New(w).With().Timestamp().Str(zerolog.CallerFieldName, name).Logger().Level(level)
}
