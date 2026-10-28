package dto

import "github.com/perber/wiki/internal/core/auth"

// UserLabel is the public label of a user: the ID plus the display username.
// It keeps the core auth type out of the HTTP contract.
type UserLabel struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// ResolveUserLabel returns the label for userID, or nil when there is no
// resolver, the ID is empty or the user cannot be resolved. The label is an
// optional decoration of a response: an unknown user is expected (deleted
// authors) and a lookup error must not fail the request, so the error is
// dropped here without logging, as the callers did before.
func ResolveUserLabel(r *auth.UserResolver, userID string) *UserLabel {
	if r == nil {
		return nil
	}
	label, err := r.ResolveUserLabel(userID)
	if err != nil || label == nil {
		return nil
	}
	return &UserLabel{ID: label.ID, Username: label.Username}
}
