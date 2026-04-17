package vk

import (
	"testing"
	"wlgposter/internal/post"
)

func TestCanPublish_AllowsPostWithoutMedia(t *testing.T) {
	if !CanPublish(&post.Post{}) {
		t.Fatal("expected post without media to be publishable")
	}
}

func TestCanPublish_BlocksSingleAudioOrVoice(t *testing.T) {
	tests := []string{"audio", "voice"}
	for _, mediaType := range tests {
		p := &post.Post{
			Media: []*post.Media{{Type: mediaType}},
		}
		if CanPublish(p) {
			t.Fatalf("expected single %s media post to be blocked", mediaType)
		}
	}
}

func TestCanPublish_AllowsPhotoAndMixedMedia(t *testing.T) {
	photoPost := &post.Post{
		Media: []*post.Media{{Type: "photo"}},
	}
	if !CanPublish(photoPost) {
		t.Fatal("expected single photo media post to be publishable")
	}

	mixedPost := &post.Post{
		Media: []*post.Media{
			{Type: "audio"},
			{Type: "photo"},
		},
	}
	if !CanPublish(mixedPost) {
		t.Fatal("expected mixed media post to be publishable")
	}
}
