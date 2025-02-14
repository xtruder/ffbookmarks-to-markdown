package firefox

import (
	"strings"

	"github.com/xtruder/ffbookmarks-to-markdown/internal/bookmarks"
)

// BookmarksRoot represents the root JSON structure from ffsclient
type BookmarksRoot struct {
	Bookmarks struct {
		Menu    bookmarks.Bookmark `json:"menu"`
		Mobile  bookmarks.Bookmark `json:"mobile"`
		Toolbar bookmarks.Bookmark `json:"toolbar"`
		Unfiled bookmarks.Bookmark `json:"unfiled"`
	} `json:"bookmarks"`
	Missing      []string `json:"missing"`
	Unreferenced []string `json:"unreferenced"`
}

func (root *BookmarksRoot) Path(path string) *bookmarks.Bookmark {
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "menu":
		return root.Bookmarks.Menu.Path(path)
	case "mobile":
		return root.Bookmarks.Mobile.Path(path)
	case "toolbar":
		return root.Bookmarks.Toolbar.Path(path)
	}

	return nil
}
