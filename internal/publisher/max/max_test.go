package max

import (
	"context"
	"errors"
	"testing"
	"time"
	"wlgposter/internal/utils"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
)

func TestRetryMediaUploadRetriesOnAnyAPIError(t *testing.T) {
	t.Cleanup(func() {
		sleep = time.Sleep
		utils.WaitForRetry = utilsWaitForRetry
	})
	sleep = func(time.Duration) {}
	utils.WaitForRetry = func(context.Context, time.Duration) error { return nil }

	attempts := 0
	result, err := utils.RetryMediaUpload(
		context.Background(),
		utils.RetryMediaUploadOptions{
			Platform: "max",
			Type:     "video",
			FileName: "video.mp4",
			Path:     `tmp\video.mp4`,
		},
		func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", &maxbot.APIError{Code: 400, Message: "bad.request"}
			}
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("RetryMediaUpload returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("RetryMediaUpload result = %q, want ok", result)
	}
	if attempts != 3 {
		t.Fatalf("RetryMediaUpload attempts = %d, want 3", attempts)
	}
}

func TestRetryMediaUploadRetriesNonAPIErrorUntilLimit(t *testing.T) {
	t.Cleanup(func() {
		sleep = time.Sleep
		utils.WaitForRetry = utilsWaitForRetry
	})
	sleep = func(time.Duration) {}
	utils.WaitForRetry = func(context.Context, time.Duration) error { return nil }

	wantErr := errors.New("disk open failed")
	attempts := 0
	_, err := utils.RetryMediaUpload(
		context.Background(),
		utils.RetryMediaUploadOptions{
			Platform: "max",
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

func TestRetryMediaUploadRetriesOnPlainHTTPError(t *testing.T) {
	t.Cleanup(func() {
		sleep = time.Sleep
		utils.WaitForRetry = utilsWaitForRetry
	})
	sleep = func(time.Duration) {}
	utils.WaitForRetry = func(context.Context, time.Duration) error { return nil }

	attempts := 0
	_, err := utils.RetryMediaUpload(
		context.Background(),
		utils.RetryMediaUploadOptions{
			Platform: "max",
			Type:     "photo",
			FileName: "photo.jpg",
			Path:     `tmp\photo.jpg`,
		},
		func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("upload: HTTP 400: Bad Request")
			}
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("RetryMediaUpload returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("RetryMediaUpload attempts = %d, want 3", attempts)
	}
}

var utilsWaitForRetry = utils.WaitForRetry
