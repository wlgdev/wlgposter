package observability

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"wlgposter/internal/config"
)

const (
	shutdownTimeout = 5 * time.Second
	productionEnv   = "production"
)

type shutdowner interface {
	Shutdown(context.Context) error
}

func Init(ctx context.Context, cfg *config.Config) (zerolog.Logger, func(context.Context) error, error) {
	var (
		mirror     *mirrorWriter
		shutdowns  []shutdowner
		initErrors []error
	)

	if isProduction(cfg) {
		logProvider, logMirror, meterProvider, err := initOTEL(ctx, cfg)
		if err != nil {
			initErrors = append(initErrors, err)
		}
		if logMirror != nil {
			mirror = logMirror
		}
		if logProvider != nil {
			shutdowns = append(shutdowns, logProvider)
		}
		if meterProvider != nil {
			shutdowns = append(shutdowns, meterProvider)
		}
	}

	var extraWriters []io.Writer
	if mirror != nil {
		extraWriters = append(extraWriters, mirror)
	}

	logger := newLogger(cfg, extraWriters...)
	shutdown := func(ctx context.Context) error {
		if mirror != nil {
			mirror.Flush()
		}

		var errs []error
		for i := len(shutdowns) - 1; i >= 0; i-- {
			if err := shutdowns[i].Shutdown(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	return logger, shutdown, errors.Join(initErrors...)
}

func ShutdownTimeout() time.Duration {
	return shutdownTimeout
}

func newLogger(cfg *config.Config, extraWriters ...io.Writer) zerolog.Logger {
	level := zerolog.InfoLevel
	if cfg != nil && strings.TrimSpace(cfg.LogLevel) != "" {
		if parsed, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel)); err == nil {
			level = parsed
		}
	}

	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339

	writers := []io.Writer{consoleWriter(os.Stdout)}
	for _, writer := range extraWriters {
		if writer != nil {
			writers = append(writers, consoleWriter(writer))
		}
	}

	logger := zerolog.New(zerolog.MultiLevelWriter(writers...)).With().Timestamp().Logger()
	log.Logger = logger
	return logger
}

func consoleWriter(out io.Writer) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}
}

func isProduction(cfg *config.Config) bool {
	return cfg != nil && strings.EqualFold(cfg.ENV, productionEnv)
}
