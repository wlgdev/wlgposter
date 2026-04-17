package post

import "testing"

func TestDownloadAllMedia_SkipsMediaWithMaxToken(t *testing.T) {
	downloadCalled := false
	p := &Post{
		Media: []*Media{{
			Type:     "photo",
			FileId:   "file-1",
			FileName: "image.jpg",
			Size:     1024,
			MaxToken: "cached-token",
			DownloadFn: func(path string) (string, error) {
				downloadCalled = true
				return path, nil
			},
		}},
	}

	if errs := p.DownloadAllMedia(t.TempDir(), 10); len(errs) != 0 {
		t.Fatalf("DownloadAllMedia() errors = %v", errs)
	}

	if downloadCalled {
		t.Fatal("expected DownloadFn to be skipped for cached media")
	}
}
