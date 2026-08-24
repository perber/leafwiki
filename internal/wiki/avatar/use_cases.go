package avatar

import (
	"context"
	"mime/multipart"

	coreavatar "github.com/perber/wiki/internal/avatar"
)

// ─── UploadAvatarUseCase ─────────────────────────────────────────────────────

type UploadAvatarUseCase struct {
	avatars *coreavatar.AvatarService
}

func NewUploadAvatarUseCase(a *coreavatar.AvatarService) *UploadAvatarUseCase {
	return &UploadAvatarUseCase{avatars: a}
}

func (uc *UploadAvatarUseCase) Execute(_ context.Context, userID string, file multipart.File, filename string) error {
	return uc.avatars.UploadAvatar(userID, file, filename)
}

// ─── DeleteAvatarUseCase ─────────────────────────────────────────────────────

type DeleteAvatarUseCase struct {
	avatars *coreavatar.AvatarService
}

func NewDeleteAvatarUseCase(a *coreavatar.AvatarService) *DeleteAvatarUseCase {
	return &DeleteAvatarUseCase{avatars: a}
}

func (uc *DeleteAvatarUseCase) Execute(_ context.Context, userID string) error {
	return uc.avatars.DeleteAvatar(userID)
}
