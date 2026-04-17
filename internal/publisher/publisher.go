package publisher

import "wlgposter/internal/post"

type Options struct {
	ReplyID string
}

type Client interface {
	Create(p *post.Post, opts Options) (string, []error)
	Edit(platformID string, p *post.Post, opts Options) (string, bool, []error)
	Delete(platformID string) error
}
