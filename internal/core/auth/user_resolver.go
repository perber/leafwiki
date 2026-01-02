package auth

import (
	"log"
	"sync"
)

type UserLabel struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type UserResolver struct {
	userService *UserService
	resolved    map[string]*UserLabel
	mu          sync.RWMutex
}

func NewUserResolver(userService *UserService) (*UserResolver, error) {
	users, err := userService.GetUsers() // preload users
	if err != nil {
		log.Println("Failed to preload users for UserResolver:", err)
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
	user, err := r.userService.GetUserByID(userID)
	if err != nil {
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
	users, err := r.userService.GetUsers()
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
