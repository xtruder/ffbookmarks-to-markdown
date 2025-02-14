// Firefox bookmark fetching and parsing
// Contains: getFirefoxBookmarks, findFolder, Bookmark struct

package firefox

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// FirefoxFetcher handles fetching bookmarks from Firefox
type FirefoxFetcher struct {
	FFSyncCmd string
}

// NewFirefoxFetcher creates a new Firefox bookmarks fetcher
func NewFirefoxFetcher() *FirefoxFetcher {
	return &FirefoxFetcher{FFSyncCmd: "ffsclient"}
}

// GetBookmarks fetches all bookmarks from Firefox
func (f *FirefoxFetcher) GetBookmarks() (*BookmarksRoot, error) {
	cmd := exec.Command(f.FFSyncCmd, "bookmarks", "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute ffsclient: %w", err)
	}

	var root BookmarksRoot
	if err := json.Unmarshal(output, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &root, nil
}

// FindFolder finds a folder by name in the bookmark tree
// func (f *FirefoxFetcher) FindFolder(bookmarks []Bookmark, name string) *Bookmark {
// 	for _, b := range bookmarks {
// 		if b.Type == "folder" {
// 			if b.Title == name {
// 				return &b
// 			}
// 			if len(b.Children) > 0 {
// 				if found := f.FindFolder(b.Children, name); found != nil {
// 					return found
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// // DisplayFolders prints the folder structure with proper indentation
// func (f *FirefoxFetcher) DisplayFolders(bookmarks []Bookmark, level int) {
// 	indent := strings.Repeat("  ", level)
// 	for _, b := range bookmarks {
// 		if b.Type == "folder" {
// 			fmt.Printf("%s- %s\n", indent, b.Title)
// 			if len(b.Children) > 0 {
// 				f.DisplayFolders(b.Children, level+1)
// 			}
// 		}
// 	}
// }
