package vk

import "wlgposter/internal/post"

func CanPublish(p *post.Post) bool {
	if p == nil || len(p.Media) != 1 {
		return true
	}

	switch p.Media[0].Type {
	case "audio", "voice":
		return false
	default:
		return true
	}
}
