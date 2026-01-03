package tools

import "github.com/perber/wiki/internal/core/auth"

func ResetAdminPassword(storageDir string) (*auth.User, error) {
	store, err := auth.NewUserStore(storageDir)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	userService := auth.NewUserService(store)
	return userService.ResetAdminUserPassword()
}
