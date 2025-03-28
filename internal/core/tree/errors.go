package tree

import "errors"

var ErrPageNotFound = errors.New("page not found")
var ErrParentNotFound = errors.New("parent not found")
var ErrTreeNotLoaded = errors.New("tree not loaded")
var ErrPageHasChildren = errors.New("page has children")
