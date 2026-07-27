import { getAssets, uploadAsset } from '@/lib/api/assets'
import { mapApiError } from '@/lib/api/errors'
import { formatBytes } from '@/lib/config'
import { useConfigStore } from '@/stores/config'
import { UploadCloud } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('assets')
  const [assets, setAssets] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const fileInput = useRef<HTMLInputElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [isHovered, setIsHovered] = useState(false)
  const [editingFilename, setEditingFilename] = useState<string | null>(null)
  const [uploadingFiles, setUploadingFiles] = useState<Set<string>>(new Set())
  const maxAssetUploadSizeBytes = useConfigStore(
    (s) => s.maxAssetUploadSizeBytes,
  )

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
    if (file.size > maxAssetUploadSizeBytes) {
      toast.error(
        t('manager.fileTooLarge', {
          maxSize: formatBytes(maxAssetUploadSizeBytes),
        }),
      )
      return
    }

    setUploadingFiles((prev) => new Set(prev).add(file.name))

    try {
      await uploadAsset(pageId, file)
      await loadAssets(false)
      onAssetVersionChange?.()
    } catch (err) {
      console.error('Upload failed', err)
      toast.error(mapApiError(err, t('manager.uploadErrorFallback')).message)
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
      <div className="asset-manager__title">{t('manager.title')}</div>

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
          {t('manager.dropzoneText')}
        </p>
        <p className="asset-manager__dropzone-text">
          {t('manager.maxFileSize', {
            maxSize: formatBytes(maxAssetUploadSizeBytes),
          })}
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
            {t('manager.uploading', { count: uploadingFiles.size })}
          </div>
        )}
      </div>

      <div className="asset-manager__list-container">
        {loading ? (
          <p className="asset-manager__loading">{t('manager.loading')}</p>
        ) : assets.length === 0 ? (
          <p className="asset-manager__empty">{t('manager.empty')}</p>
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

      <p className="asset-manager__tip">{t('manager.tip')}</p>
    </div>
  )
}
