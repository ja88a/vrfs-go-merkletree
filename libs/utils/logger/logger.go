package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Interface -.
type Interface interface {
	Debug(message interface{}, args ...interface{})
	Info(message string, args ...interface{})
	Warn(message string, args ...interface{})
	Error(message interface{}, args ...interface{})
	Fatal(message interface{}, args ...interface{})
}

// Logger -.
type Logger struct {
	logger *zerolog.Logger
}

var _ Interface = (*Logger)(nil)

// New -.
func New(level string) *Logger {
	var l zerolog.Level

	switch strings.ToLower(level) {
	case "error":
		l = zerolog.ErrorLevel
	case "warn":
		l = zerolog.WarnLevel
	case "info":
		l = zerolog.InfoLevel
	case "debug":
		l = zerolog.DebugLevel
	default:
		l = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(l)

	skipFrameCount := 3
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + skipFrameCount).Logger()

	return &Logger{
		logger: &logger,
	}
}

// Debug -.
func (l *Logger) Debug(message interface{}, args ...interface{}) {
	l.msg("debug", message, args...)
}

// Info -.
func (l *Logger) Info(message string, args ...interface{}) {
	l.msg("info", message, args...)
}

// Warn -.
func (l *Logger) Warn(message string, args ...interface{}) {
	l.msg("warn", message, args...)
}

// Error -.
func (l *Logger) Error(message interface{}, args ...interface{}) {
	l.msg("error", message, args...)
}

// Fatal -.
func (l *Logger) Fatal(message interface{}, args ...interface{}) {
	l.msg("fatal", message, args...)

	os.Exit(1)
}

func (l *Logger) msg(level string, message interface{}, args ...interface{}) {
	switch msg := message.(type) {
	case error:
		l.log(level, msg.Error(), args...)
	case string:
		l.log(level, msg, args...)
	default:
		l.log("error", fmt.Sprintf("%s message %v has unknown type %v", level, message, msg), args...)
	}
}

func (l *Logger) log(level string, message string, args ...interface{}) {
	switch level {
	case "debug":
		if len(args) == 0 {
			l.logger.Debug().Msg(message)
		} else {
			l.logger.Debug().Msgf(message, args...)
		}
	case "info":
		if len(args) == 0 {
			l.logger.Info().Msg(message)
		} else {
			l.logger.Info().Msgf(message, args...)
		}
	case "warn":
		if len(args) == 0 {
			l.logger.Warn().Msg(message)
		} else {
			l.logger.Warn().Msgf(message, args...)
		}
	case "error":
		if len(args) == 0 {
			l.logger.Error().Msg(message)
		} else {
			l.logger.Error().Msgf(message, args...)
		}
	case "fatal":
		if len(args) == 0 {
			l.logger.Fatal().Msg(message)
		} else {
			l.logger.Fatal().Msgf(message, args...)
		}
	default:
		l.logger.Error().Msg(fmt.Sprintf("Unsupporter logging level `%v` for message `%v`", level, message))
	}
}
