package tree

import "errors"

var ErrPageNotFound = errors.New("page not found")
var ErrParentNotFound = errors.New("parent not found")
var ErrTreeNotLoaded = errors.New("tree not loaded")
var ErrPageHasChildren = errors.New("page has children")
var ErrPageAlreadyExists = errors.New("page already exists")
var ErrMovePageCircularReference = errors.New("circular reference detected")
var ErrPageCannotBeMovedToItself = errors.New("page cannot be moved to itself")
var ErrInvalidSortOrder = errors.New("invalid sort order")
var ErrFrontmatterParse = errors.New("frontmatter parse error")
var ErrFileNotFound = errors.New("file not found")
