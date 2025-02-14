package bookmarks

import (
	"iter"
	"slices"
	"strings"
)

// Bookmark represents a Firefox bookmark
type Bookmark struct {
	Added     string     `json:"added"`
	AddedUnix int64      `json:"added_unix"`
	Deleted   bool       `json:"deleted"`
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Type      string     `json:"type"`
	URI       string     `json:"uri,omitempty"`
	Children  []Bookmark `json:"children,omitempty"`
}

func (folder Bookmark) All() iter.Seq2[string, *Bookmark] {
	return func(yield func(string, *Bookmark) bool) {
		var collect func(b Bookmark, path string)
		collect = func(b Bookmark, path string) {
			yield(path, &b)

			for _, child := range b.Children {
				if path == "" {
					collect(child, child.Title)
				} else {
					collect(child, path+"/"+child.Title)
				}
			}
		}

		collect(folder, folder.Title)
	}
}

func (folder *Bookmark) Path(path string) *Bookmark {
	parts := strings.Split(path, "/")
	return folder.path(parts...)
}

func (folder *Bookmark) path(parts ...string) *Bookmark {
	if len(parts) == 0 {
		return nil
	}

	if folder.Title != parts[0] {
		return nil
	}

	if len(parts) == 1 {
		return folder
	}

	idx := slices.IndexFunc(folder.Children, func(c Bookmark) bool {
		return c.Title == parts[1]
	})

	if idx == -1 {
		return nil
	}

	return folder.Children[idx].path(parts[1:]...)
}
