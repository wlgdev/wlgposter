package vk

import (
	"context"
	"errors"
	"testing"
	"time"
	"wlgposter/internal/utils"
)

func TestRetryMediaUploadRetriesNonAPIErrorUntilLimit(t *testing.T) {
	t.Cleanup(func() {
		utils.WaitForRetry = utilsWaitForRetry
	})
	utils.WaitForRetry = func(context.Context, time.Duration) error { return nil }

	wantErr := errors.New("temporary disk open failed")
	attempts := 0
	_, err := utils.RetryMediaUpload(
		context.Background(),
		utils.RetryMediaUploadOptions{
			Platform: "vk",
			Type:     "video",
			FileName: "video.mp4",
			Path:     `tmp\video.mp4`,
		},
		func() (string, error) {
			attempts++
			return "", wantErr
		},
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("RetryMediaUpload error = %v, want %v", err, wantErr)
	}
	if attempts != utils.MediaUploadMaxRetries {
		t.Fatalf("RetryMediaUpload attempts = %d, want %d", attempts, utils.MediaUploadMaxRetries)
	}
}

var utilsWaitForRetry = utils.WaitForRetry
