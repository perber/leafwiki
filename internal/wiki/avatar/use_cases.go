package avatar

import (
	"context"
	"mime/multipart"

	coreavatar "github.com/perber/wiki/internal/avatar"
)

// ─── UploadAvatarUseCase ─────────────────────────────────────────────────────

// UploadAvatarInput is the input for UploadAvatarUseCase.
type UploadAvatarInput struct {
	UserID string
	File   multipart.File
	// Filename is the client-supplied name of the upload; its extension must be an allowed image type.
	Filename string
}

// UploadAvatarUseCase stores an uploaded image as the user's avatar.
type UploadAvatarUseCase struct {
	avatars *coreavatar.AvatarService
}

// NewUploadAvatarUseCase constructs an UploadAvatarUseCase.
func NewUploadAvatarUseCase(a *coreavatar.AvatarService) *UploadAvatarUseCase {
	return &UploadAvatarUseCase{avatars: a}
}

// Execute stores the uploaded image as the avatar of in.UserID.
func (uc *UploadAvatarUseCase) Execute(_ context.Context, in UploadAvatarInput) error {
	return uc.avatars.UploadAvatar(in.UserID, in.File, in.Filename)
}

// ─── DeleteAvatarUseCase ─────────────────────────────────────────────────────

// DeleteAvatarInput is the input for DeleteAvatarUseCase.
type DeleteAvatarInput struct {
	UserID string
}

// DeleteAvatarUseCase removes the user's avatar.
type DeleteAvatarUseCase struct {
	avatars *coreavatar.AvatarService
}

// NewDeleteAvatarUseCase constructs a DeleteAvatarUseCase.
func NewDeleteAvatarUseCase(a *coreavatar.AvatarService) *DeleteAvatarUseCase {
	return &DeleteAvatarUseCase{avatars: a}
}

// Execute removes the avatar of in.UserID; deleting a missing avatar is not an error.
func (uc *DeleteAvatarUseCase) Execute(_ context.Context, in DeleteAvatarInput) error {
	return uc.avatars.DeleteAvatar(in.UserID)
}
