package utils

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	MediaUploadMaxRetries = 5
	MediaUploadRetryDelay = 2 * time.Second
)

var WaitForRetry = waitForRetry

type RetryMediaUploadOptions struct {
	Platform string
	Type     string
	FileName string
	Path     string
}

func RetryMediaUpload[T any](
	ctx context.Context,
	opts RetryMediaUploadOptions,
	upload func() (T, error),
) (T, error) {
	var zero T

	for attempt := 1; attempt <= MediaUploadMaxRetries; attempt++ {
		result, err := upload()
		if err == nil {
			return result, nil
		}

		log.Warn().
			Err(err).
			Str("platform", opts.Platform).
			Str("type", opts.Type).
			Str("name", opts.FileName).
			Str("path", opts.Path).
			Int("attempt", attempt).
			Int("max_attempts", MediaUploadMaxRetries).
			Dur("retry_in", MediaUploadRetryDelay).
			Msg("Media upload failed, retrying")

		if attempt >= MediaUploadMaxRetries {
			return zero, err
		}

		if err := WaitForRetry(ctx, MediaUploadRetryDelay); err != nil {
			return zero, err
		}
	}

	return zero, nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
