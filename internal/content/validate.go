package content

// ValidatePost parses a single post file exactly as the server does and
// returns the parsed post, or an error describing why the server would
// reject it. filename is used only in error messages.
//
// It is intended for offline tooling (for example the local Blog Studio)
// that must never write a post the running blog-server cannot index.
func ValidatePost(data []byte, filename string) (*Post, error) {
	return parsePost(string(data), filename)
}

// ValidateDir validates a whole content root (the directory that contains
// posts/, assets/ and demos/) using the same indexing code as
// FolderStore.Reload. A nil error means a blog-server pointed at root would
// load successfully. This catches problems a per-file check cannot, such as
// duplicate slugs across posts.
func ValidateDir(root string) error {
	_, _, err := loadPosts(root)
	return err
}
