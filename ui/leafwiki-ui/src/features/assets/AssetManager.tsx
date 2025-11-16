import { getAssets, uploadAsset } from '@/lib/api/assets'
import { MAX_UPLOAD_SIZE, MAX_UPLOAD_SIZE_MB } from '@/lib/config'
import { UploadCloud } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { AssetItem } from './AssetItem'

type Props = {
  pageId: string
  onInsert?: (md: string) => void // optional callback for markdown insertion
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
    if (filename) {
      isRenamingRef.current = true
    } else {
      isRenamingRef.current = false
    }
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
      if (onAssetVersionChange) {
        onAssetVersionChange()
      }
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

  return (
    <div className="space-y-3 text-sm">
      <div className="font-semibold text-gray-700">Assets</div>
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
        className={`flex cursor-pointer flex-col items-center justify-center rounded-md border border-dashed p-4 text-center text-gray-500 transition ${
          isDragging
            ? 'border-blue-400 bg-blue-50 text-blue-600'
            : isHovered
              ? 'border-gray-300 bg-gray-50'
              : 'border-gray-200 hover:bg-gray-50'
        }`}
      >
        <UploadCloud className="mb-2" size={20} />
        <p className="text-xs">Drop files here or click to upload</p>
        <input
          type="file"
          ref={fileInput}
          onChange={handleUpload}
          className="hidden"
          multiple
        />
        {uploadingFiles.size > 0 && (
          <div className="text-xs text-blue-600">
            Uploading {uploadingFiles.size} file
            {uploadingFiles.size > 1 ? 's' : ''}…
          </div>
        )}
      </div>
      <div className="h-96">
        {loading ? (
          <p className="text-xs text-gray-500">Loading assets…</p>
        ) : assets.length === 0 ? (
          <p className="text-xs text-gray-400 italic">No assets yet</p>
        ) : (
          <ul className="custom-scrollbar h-full space-y-2 overflow-y-auto">
            {assets.map((filename) => (
              <AssetItem
                key={filename}
                filename={filename}
                editingFilename={editingFilename}
                setEditingFilename={handleSetEditingFilename}
                pageId={pageId}
                onReload={loadAssets}
                onAssetVersionChange={onAssetVersionChange}
                onInsert={(md) => {
                  onInsert?.(md)
                }}
                onFilenameChange={onFilenameChange}
              />
            ))}
          </ul>
        )}
      </div>
      <p className="mt-2 text-xs text-gray-500 italic">
        Tip: Double-click on an asset to insert it into the page.
      </p>
    </div>
  )
}
