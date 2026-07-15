import { lazy } from 'react'

export const UnsavedChangesDialog = lazy(() =>
  import('@/components/UnsavedChangesDialog').then((m) => ({
    default: m.UnsavedChangesDialog,
  })),
)
export const AssetManagerDialog = lazy(() =>
  import('@/features/assets/AssetManagerDialog').then((m) => ({
    default: m.AssetManagerDialog,
  })),
)
export const WikiLinkDisambiguationDialog = lazy(() =>
  import('@/features/wikilinks/WikiLinkDisambiguationDialog').then((m) => ({
    default: m.WikiLinkDisambiguationDialog,
  })),
)
export const LinkInsertDialog = lazy(() =>
  import('@/features/editor/LinkInsertDialog').then((m) => ({
    default: m.LinkInsertDialog,
  })),
)
export const ImagePreviewDialog = lazy(() =>
  import('@/features/imagepreview/ImagePreviewDialog').then((m) => ({
    default: m.ImagePreviewDialog,
  })),
)
export const RestoreRevisionDialog = lazy(() =>
  import('@/features/history/RestoreRevisionDialog').then((m) => ({
    default: m.RestoreRevisionDialog,
  })),
)
export const PageQuickSwitcherDialog = lazy(() =>
  import('@/features/page-switcher/PageQuickSwitcherDialog').then((m) => ({
    default: m.PageQuickSwitcherDialog,
  })),
)
export const AddPageDialog = lazy(() =>
  import('@/features/page/AddPageDialog').then((m) => ({
    default: m.AddPageDialog,
  })),
)
export const CopyPageDialog = lazy(() =>
  import('@/features/page/CopyPageDialog').then((m) => ({
    default: m.CopyPageDialog,
  })),
)
export const CreatePageByPathDialog = lazy(() =>
  import('@/features/page/CreatePageByPathDialog').then((m) => ({
    default: m.CreatePageByPathDialog,
  })),
)
export const DeletePageDialog = lazy(() =>
  import('@/features/page/DeletePageDialog').then((m) => ({
    default: m.DeletePageDialog,
  })),
)
export const EditPageMetadataDialog = lazy(() =>
  import('@/features/page/EditPageMetadataDialog').then((m) => ({
    default: m.EditPageMetadataDialog,
  })),
)
export const MovePageDialog = lazy(() =>
  import('@/features/page/MovePageDialog').then((m) => ({
    default: m.MovePageDialog,
  })),
)
export const PermalinkDialog = lazy(
  () => import('@/features/page/PermalinkDialog'),
)
export const PageRefactorDialog = lazy(() =>
  import('@/features/page/PageRefactorDialog').then((m) => ({
    default: m.PageRefactorDialog,
  })),
)
export const SortPagesDialog = lazy(() =>
  import('@/features/page/SortPagesDialog').then((m) => ({
    default: m.SortPagesDialog,
  })),
)
export const ShortcutsDialog = lazy(() =>
  import('@/features/shortcuts/ShortcutsDialog').then((m) => ({
    default: m.ShortcutsDialog,
  })),
)
export const Search = lazy(() => import('@/features/search/Search'))
export const ChangeOwnPasswordDialog = lazy(() =>
  import('@/features/users/ChangeOwnPasswordDialog').then((m) => ({
    default: m.ChangeOwnPasswordDialog,
  })),
)
export const ChangePasswordDialog = lazy(() =>
  import('@/features/users/ChangePasswordDialog').then((m) => ({
    default: m.ChangePasswordDialog,
  })),
)
export const DeleteUserDialog = lazy(() =>
  import('@/features/users/DeleteUserDialog').then((m) => ({
    default: m.DeleteUserDialog,
  })),
)
export const UserFormDialog = lazy(() =>
  import('@/features/users/UserFormDialog').then((m) => ({
    default: m.UserFormDialog,
  })),
)
export const TOTPSetupDialog = lazy(() =>
  import('@/features/users/TOTPSetupDialog').then((m) => ({
    default: m.TOTPSetupDialog,
  })),
)
export const TOTPDisableDialog = lazy(() =>
  import('@/features/users/TOTPDisableDialog').then((m) => ({
    default: m.TOTPDisableDialog,
  })),
)
