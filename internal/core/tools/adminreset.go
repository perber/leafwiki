package tools

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/auth"
)

func ResetAdminPassword(storageDir string) (*auth.User, error) {
	store, err := auth.NewUserStore(storageDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Default().Error("could not close store", "error", err)
		}
	}()

	userService := auth.NewUserService(store)
	return userService.ResetAdminUserPassword()
}
