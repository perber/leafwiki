package tree

// PathLookup helpers for LookupPath()
type PathSegment struct {
	Slug   string  `json:"slug"`
	Exists bool    `json:"exists"`
	ID     *string `json:"id,omitempty"`
}

type PathLookup struct {
	Path     string        `json:"path"`
	Segments []PathSegment `json:"segments"`
	Exists   bool          `json:"exists"`
}
