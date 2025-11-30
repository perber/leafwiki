import { getAssets, uploadAsset } from '@/lib/api/assets'
import { MAX_UPLOAD_SIZE, MAX_UPLOAD_SIZE_MB } from '@/lib/config'
import { UploadCloud } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { AssetItem } from './AssetItem'

type Props = {
  pageId: string
  onInsert?: (md: string) => void
  onFilenameChange?: (before: string, after: string) => void
  onAssetVersionChange?: () => void
  isRenamingRef: React.RefObject<boolean>
}

export function AssetManager({
  pageId,
  onInsert,
  onFilenameChange,
  onAssetVersionChange,
  isRenamingRef,
}: Props) {
  const [assets, setAssets] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const fileInput = useRef<HTMLInputElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [isHovered, setIsHovered] = useState(false)
  const [editingFilename, setEditingFilename] = useState<string | null>(null)
  const [uploadingFiles, setUploadingFiles] = useState<Set<string>>(new Set())

  const handleSetEditingFilename = (filename: string | null) => {
    isRenamingRef.current = !!filename
    setEditingFilename(filename)
  }

  const loadAssets = useCallback(
    async (showLoading = false) => {
      if (showLoading) setLoading(true)
      try {
        const result = await getAssets(pageId)
        setAssets(result)
      } catch (err) {
        console.error('Failed to load assets', err)
      } finally {
        if (showLoading) setLoading(false)
      }
    },
    [pageId],
  )

  useEffect(() => {
    loadAssets(true)
  }, [pageId, loadAssets])

  const handleUploadFile = async (file: File) => {
    if (file.size > MAX_UPLOAD_SIZE) {
      toast.error(`File too large. Max ${MAX_UPLOAD_SIZE_MB}MB allowed.`)
      return
    }

    setUploadingFiles((prev) => new Set(prev).add(file.name))

    try {
      await uploadAsset(pageId, file)
      await loadAssets(false)
      onAssetVersionChange?.()
    } catch (err) {
      console.error('Upload failed', err)
    } finally {
      setUploadingFiles((prev) => {
        const next = new Set(prev)
        next.delete(file.name)
        return next
      })
    }
  }

  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    await Promise.all(files.map(handleUploadFile))
    if (fileInput.current) fileInput.current.value = ''
  }

  const handleDrop = async (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    await Promise.all(
      Array.from(e.dataTransfer.files ?? []).map(handleUploadFile),
    )
  }

  const dropzoneClassName = [
    'asset-manager__dropzone',
    isDragging
      ? 'asset-manager__dropzone--dragging'
      : isHovered
        ? 'asset-manager__dropzone--hover'
        : '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div className="asset-manager">
      <div className="asset-manager__title">Assets</div>

      <div
        data-testid="asset-upload-dropzone"
        ref={dropRef}
        onDragOver={(e) => {
          e.preventDefault()
          setIsDragging(true)
        }}
        onDragEnter={() => setIsDragging(true)}
        onDragLeave={() => {
          setIsDragging(false)
          setIsHovered(false)
        }}
        onDrop={handleDrop}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        onClick={() => fileInput.current?.click()}
        className={dropzoneClassName}
      >
        <UploadCloud className="asset-manager__dropzone-icon" size={20} />
        <p className="asset-manager__dropzone-text">
          Drop files here or click to upload
        </p>
        <input
          type="file"
          ref={fileInput}
          onChange={handleUpload}
          className="hidden"
          multiple
        />
        {uploadingFiles.size > 0 && (
          <div className="asset-manager__dropzone-uploading">
            Uploading {uploadingFiles.size} file
            {uploadingFiles.size > 1 ? 's' : ''}…
          </div>
        )}
      </div>

      <div className="asset-manager__list-container">
        {loading ? (
          <p className="asset-manager__loading">Loading assets…</p>
        ) : assets.length === 0 ? (
          <p className="asset-manager__empty">No assets yet</p>
        ) : (
          <ul className="asset-manager__list custom-scrollbar">
            {assets.map((filename) => (
              <AssetItem
                key={filename}
                filename={filename}
                editingFilename={editingFilename}
                setEditingFilename={handleSetEditingFilename}
                pageId={pageId}
                onReload={loadAssets}
                onAssetVersionChange={onAssetVersionChange}
                onInsert={(md) => onInsert?.(md)}
                onFilenameChange={onFilenameChange}
              />
            ))}
          </ul>
        )}
      </div>

      <p className="asset-manager__tip">
        Tip: Double-click on an asset to insert it into the page.
      </p>
    </div>
  )
}
