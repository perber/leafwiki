import { getAssets, uploadAsset } from '@/lib/api'
import { UploadCloud } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { AssetItem } from './AssetItem'

type Props = {
  pageId: string
  onInsert?: (md: string) => void // optionaler Callback fürs Markdown
  isRenamingRef: React.RefObject<boolean>
}

export function AssetManager({ pageId, onInsert, isRenamingRef }: Props) {
  const [assets, setAssets] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const fileInput = useRef<HTMLInputElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [isHovered, setIsHovered] = useState(false)


  const loadAssets = useCallback(async () => {
    setLoading(true)
    try {
      const result = await getAssets(pageId)
      setAssets(result)
    } catch (err) {
      console.error('Failed to load assets', err)
    } finally {
      setLoading(false)
    }
  }, [pageId])

  useEffect(() => {
    loadAssets()
  }, [pageId, loadAssets])

  const handleUploadFile = async (file: File) => {
    const MAX_UPLOAD_SIZE_MB = 50
    const MAX_UPLOAD_SIZE = MAX_UPLOAD_SIZE_MB * 1024 * 1024

    if (file.size > MAX_UPLOAD_SIZE) {
      toast.error(`File too large. Max ${MAX_UPLOAD_SIZE_MB}MB allowed.`)
      return
    }

    try {
      await uploadAsset(pageId, file)
      await loadAssets()
    } catch (err) {
      console.error('Upload failed', err)
    }
  }

  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) await handleUploadFile(file)
    if (fileInput.current) fileInput.current.value = ''
  }


  const handleDrop = async (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    const file = e.dataTransfer.files?.[0]
    if (file) await handleUploadFile(file)
  }

  return (
    <div className="space-y-3 text-sm">
      <div className="font-semibold text-gray-700">Assets</div>
      <div
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
        className={`flex cursor-pointer flex-col items-center justify-center rounded-md border border-dashed p-4 text-center text-gray-500 transition ${isDragging
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
        />
      </div>

      {loading ? (
        <p className="text-xs text-gray-500">Loading assets…</p>
      ) : assets.length === 0 ? (
        <p className="text-xs italic text-gray-400">No assets yet</p>
      ) : (
        <ul className="space-y-2">
          {assets.map((filename) => (
            <AssetItem
              key={filename}
              filename={filename}
              pageId={pageId}
              onReload={loadAssets}
              onInsert={(md) => onInsert?.(md)}
              isRenamingRef={isRenamingRef}
            />
          ))}
        </ul>
      )}
      <p className="mt-2 text-xs italic text-gray-500">
        Tip: Double-click on an asset to insert it into the page.
      </p>
    </div>
  )
}
