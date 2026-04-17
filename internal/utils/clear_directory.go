package utils

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

func clearDirectory(ctx context.Context, dir string) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				if removeErr := os.Remove(path); removeErr != nil {
					return removeErr
				}
			}
			return nil
		})
		return err
	}
}

func DailyClearDirectory(ctx context.Context, dir string, hour int) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}

		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if err := clearDirectory(ctx, dir); err != nil {
				log.Error().Err(err).Msg("Clear directory failed")
			} else {
				log.Info().Str("directory", dir).Msg("Clear directory success")
			}
		}
	}
}
