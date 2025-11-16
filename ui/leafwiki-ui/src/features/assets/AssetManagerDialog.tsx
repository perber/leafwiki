import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { DIALOG_ASSET_MANAGER } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { AssetManager } from './AssetManager'

export type AssetManagerDialogProps = {
  pageId: string
  editorRef: React.RefObject<{
    insertAtCursor: (md: string) => void
    replaceFilenameInMarkdown?: (before: string, after: string) => void
  }>
  onAssetVersionChange: () => void
  isRenamingRef: React.RefObject<boolean>
}

export function AssetManagerDialog(props: AssetManagerDialogProps) {
  const { pageId, editorRef, onAssetVersionChange, isRenamingRef } = props
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === DIALOG_ASSET_MANAGER)

  return (
    <Dialog
      open={open}
      onOpenChange={(onOpen) => {
        if (!onOpen) {
          closeDialog()
        }
      }}
    >
      <DialogContent
        className="max-w-2xl"
        onEscapeKeyDown={(e) => {
          if (isRenamingRef.current) {
            e.preventDefault()
          } else {
            closeDialog()
            e.preventDefault()
            e.stopPropagation()
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>Asset Manager</DialogTitle>
          <DialogDescription>
            Upload or select an asset to insert into the page.
          </DialogDescription>
        </DialogHeader>
        <AssetManager
          pageId={pageId}
          onAssetVersionChange={onAssetVersionChange}
          onInsert={(md) => {
            editorRef.current?.insertAtCursor(md)
            closeDialog()
          }}
          onFilenameChange={(before, after) => {
            editorRef.current?.replaceFilenameInMarkdown?.(before, after)
          }}
          isRenamingRef={isRenamingRef}
        />
      </DialogContent>
    </Dialog>
  )
}
