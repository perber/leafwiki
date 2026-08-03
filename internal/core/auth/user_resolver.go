package auth

import (
	"errors"
	"sync"
)

type UserLabel struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type UserResolver struct {
	// userService is resolved on every call rather than cached, so ResolveUserLabel/
	// Reload automatically track a live restore's AuthService.ReplaceUserStore swap
	// instead of going stale — mirrors AuthService.UserService() itself.
	userService func() *UserService
	resolved    map[string]*UserLabel
	mu          sync.RWMutex
}

func NewUserResolver(userService func() *UserService) (*UserResolver, error) {
	users, err := userService().GetUsers() // preload users
	if err != nil {
		// Not logged here: the error is returned to the caller (wiki.go's
		// NewWiki), which propagates it up to main.go's own top-level
		// fatal-error logging — logging here too would double-log the same
		// failure (ADR-0008's "log once, at the swallow point" rule).
		return nil, err
	}
	r := &UserResolver{
		userService: userService,
		resolved:    make(map[string]*UserLabel),
	}

	for _, user := range users {
		r.resolved[user.ID] = &UserLabel{
			ID:       user.ID,
			Username: user.Username,
		}
	}

	return r, nil
}

func (r *UserResolver) ResolveUserLabel(userID string) (*UserLabel, error) {
	if userID == "" {
		return nil, nil
	}

	// fast path
	r.mu.RLock()
	if user, ok := r.resolved[userID]; ok {
		r.mu.RUnlock()
		return user, nil
	}
	r.mu.RUnlock()

	// fetch
	user, err := r.userService().GetUserByID(userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Cache the miss: genuinely no such user, so an unresolvable ID
			// won't resolve differently on a later call without a Reload().
			// Without this, any reference to a nonexistent/deleted user
			// (e.g. a page's creatorID after its author account was
			// deleted) would hit userService/the store on every call,
			// forever.
			r.mu.Lock()
			if _, ok := r.resolved[userID]; !ok {
				r.resolved[userID] = nil
			}
			r.mu.Unlock()
		}
		// A transient error (e.g. the store suspended for an in-progress
		// live restore, see GetUserByID/mapUserLookupErr) is deliberately
		// NOT cached — the next call should retry against the store rather
		// than being permanently told "not found".
		return nil, err
	}
	label := &UserLabel{ID: user.ID, Username: user.Username}

	// write path (double-check)
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.resolved[userID]; ok {
		return existing, nil
	}

	r.resolved[userID] = label
	return label, nil
}

func (r *UserResolver) Reload() error {
	users, err := r.userService().GetUsers()
	if err != nil {
		return err
	}

	newMap := make(map[string]*UserLabel, len(users))
	for _, u := range users {
		newMap[u.ID] = &UserLabel{ID: u.ID, Username: u.Username}
	}

	r.mu.Lock()
	r.resolved = newMap
	r.mu.Unlock()

	return nil
}
