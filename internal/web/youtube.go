package web

import (
	"fmt"
	"net/url"
	"strings"
)

type YouTubeFetcher struct{}

func NewYouTubeFetcher() *YouTubeFetcher {
	return &YouTubeFetcher{}
}

func (f *YouTubeFetcher) Fetch(u *url.URL) (string, error) {
	var videoID string
	switch u.Host {
	case "youtube.com", "www.youtube.com":
		if u.Path == "/watch" {
			if v := u.Query().Get("v"); v != "" {
				videoID = v
			}
		}
	case "youtu.be":
		videoID = strings.TrimPrefix(u.Path, "/")
	}

	if videoID == "" {
		return "", fmt.Errorf("could not extract video ID from URL")
	}

	return fmt.Sprintf(`<iframe width="560" height="315" src="https://www.youtube.com/embed/%s" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`, videoID), nil
}
