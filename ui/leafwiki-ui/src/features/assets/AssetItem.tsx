import { Button } from '@/components/ui/button'
import { deleteAsset, renameAsset } from '@/lib/api/assets'
import { IMAGE_EXTENSIONS } from '@/lib/config'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Check, FileText, Pencil, Trash2, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { AssetPreviewTooltip } from './AssetPreviewTooltip'

const imageExtensions = IMAGE_EXTENSIONS

type Props = {
  pageId: string
  filename: string
  editingFilename: string | null
  setEditingFilename: (filename: string | null) => void
  onAssetVersionChange?: () => void
  onReload: () => void
  onInsert: (md: string) => void
  onFilenameChange?: (before: string, after: string) => void
}

export function AssetItem({
  pageId,
  filename,
  editingFilename,
  setEditingFilename,
  onReload,
  onInsert,
  onFilenameChange,
  onAssetVersionChange,
}: Props) {
  const assetUrl = filename
  const ext = filename.split('.').pop()?.toLowerCase()
  const isImage = imageExtensions.includes(ext ?? '')
  const baseName = filename.split('/').pop() ?? filename
  const isEditing = editingFilename === filename
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  const [newName, setNewName] = useState(baseName.replace(/\.[^/.]+$/, ''))

  const handleRename = useCallback(async () => {
    try {
      const newFilename = `${newName}.${ext}`
      if (newFilename === baseName) {
        setEditingFilename(null)
        return
      }

      await renameAsset(pageId, baseName, newFilename)
      toast.success('Asset renamed')
      onFilenameChange?.(baseName, newFilename)
      onAssetVersionChange?.()
      onReload()
    } catch (err: unknown) {
      if (err instanceof Error) {
        toast.error(`Rename failed: ${err.message}`)
      } else if (typeof err === 'object' && err !== null && 'error' in err) {
        toast.error(`Rename failed: ${(err as { error: string }).error}`)
      }
    }
  }, [
    pageId,
    baseName,
    newName,
    ext,
    onReload,
    onFilenameChange,
    onAssetVersionChange,
    setEditingFilename,
  ])

  const handleDelete = async () => {
    try {
      await deleteAsset(pageId, baseName)
      toast.success('Asset deleted')
      onReload()
      onAssetVersionChange?.()
    } catch (err) {
      toast.error('Delete failed')
      console.error('Delete failed', err)
    }
  }

  const handleInsertMarkdown = () => {
    if (isEditing) return

    const markdown = isImage
      ? `![${newName}](${assetUrl})\n`
      : `[${baseName}](${assetUrl})\n`

    onInsert(markdown)
  }

  // hotkeys for rename
  useEffect(() => {
    if (!isEditing) return

    const enterHotkey: HotKeyDefinition = {
      keyCombo: 'Enter',
      enabled: true,
      mode: ['dialog'],
      action: handleRename,
    }

    const escapeHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['dialog'],
      action: () => {
        setEditingFilename(null)
        setNewName(baseName.replace(/\.[^/.]+$/, ''))
      },
    }
    registerHotkey(enterHotkey)
    registerHotkey(escapeHotkey)

    return () => {
      unregisterHotkey(enterHotkey.keyCombo)
      unregisterHotkey(escapeHotkey.keyCombo)
    }
  }, [
    isEditing,
    baseName,
    registerHotkey,
    unregisterHotkey,
    handleRename,
    setEditingFilename,
  ])

  return (
    <li
      className="group asset-item"
      onDoubleClick={handleInsertMarkdown}
      data-testid="asset-item"
    >
      <div className="flex flex-1 items-center gap-1">
        {isImage ? (
          <AssetPreviewTooltip url={assetUrl} name={baseName}>
            <img
              src={assetUrl}
              alt={baseName}
              className="asset-item__preview-image"
            />
          </AssetPreviewTooltip>
        ) : (
          <AssetPreviewTooltip url={assetUrl} name={baseName}>
            <div className="asset-item__preview-file">
              <FileText size={18} />
            </div>
          </AssetPreviewTooltip>
        )}

        {isEditing ? (
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="asset-item__filename--editing-input"
          />
        ) : (
          <span className="asset-item__filename">{baseName}</span>
        )}
      </div>

      {isEditing ? (
        <>
          <Button
            variant="ghost"
            size="icon"
            className="asset-item__action-button asset-item__action-button--save"
            onClick={handleRename}
            title="Save"
          >
            <Check size={16} />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="asset-item__action-button asset-item__action-button--cancel"
            onClick={() => {
              setEditingFilename(null)
              setNewName(baseName.replace(/\.[^/.]+$/, ''))
            }}
            title="Cancel"
          >
            <X size={16} />
          </Button>
        </>
      ) : (
        <Button
          variant="ghost"
          size="icon"
          className="asset-item__action-button"
          onClick={(e) => {
            e.stopPropagation()
            setNewName(baseName.replace(/\.[^/.]+$/, ''))
            setEditingFilename(filename)
          }}
          title="Rename"
        >
          <Pencil size={16} />
        </Button>
      )}

      <Button
        variant="ghost"
        size="icon"
        className="asset-item__action-button asset-item__action-button--delete"
        onClick={(e) => {
          e.stopPropagation()
          handleDelete()
        }}
        title="Delete"
      >
        <Trash2 size={16} />
      </Button>
    </li>
  )
}
