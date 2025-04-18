import { Button } from '@/components/ui/button'
import { deleteAsset, getAssets, uploadAsset } from '@/lib/api'
import { FileText, Trash2, UploadCloud } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { AssetPreviewTooltip } from './AssetPreviewTooltip'

const imageExtensions = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg']

function generateMarkdownLink(filename: string, url: string): string {
  const ext = url.split('.').pop()?.toLowerCase() || ''
  const isImage = imageExtensions.includes(ext)

  const baseName =
    filename
      .split('/')
      .pop()
      ?.replace(/\.[^/.]+$/, '') || 'file'

  if (isImage) {
    return `![${baseName}](${url})\n`
  } else {
    return `[${filename}](${url})\n`
  }
}

type Props = {
  pageId: string
  onInsert?: (md: string) => void // optionaler Callback fürs Markdown
}

export function AssetManager({ pageId, onInsert }: Props) {
  const [assets, setAssets] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const fileInput = useRef<HTMLInputElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [isHovered, setIsHovered] = useState(false)

  const loadAssets = async () => {
    setLoading(true)
    try {
      const result = await getAssets(pageId)
      setAssets(result)
    } catch (err) {
      console.error('Failed to load assets', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAssets()
  }, [pageId])

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

      //      if (onInsert) {
      //        const markdown = generateMarkdownLink(file.name.split('.')[0], res.file)
      //        onInsert(`${markdown}\n`)
      //      }
    } catch (err) {
      console.error('Upload failed', err)
    }
  }

  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) await handleUploadFile(file)
    if (fileInput.current) fileInput.current.value = ''
  }

  const handleDelete = async (filename: string) => {
    try {
      await deleteAsset(pageId, filename)
      await loadAssets()
    } catch (err) {
      console.error('Delete failed', err)
    }
  }

  const handleDrop = async (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    const file = e.dataTransfer.files?.[0]
    if (file) await handleUploadFile(file)
  }

  const insertMarkdown = (url: string) => {
    const name = url.split('/').pop()
    const md = generateMarkdownLink(name ?? '', url)

    onInsert?.(md)
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
        />
      </div>

      {loading ? (
        <p className="text-xs text-gray-500">Loading assets…</p>
      ) : assets.length === 0 ? (
        <p className="text-xs italic text-gray-400">No assets yet</p>
      ) : (
        <ul className="space-y-2">
          {assets.map((filename) => {
            const assetUrl = filename
            const ext = filename.split('.').pop()?.toLowerCase()
            const isImage = imageExtensions.includes(ext ?? '')
            const baseName = filename.split('/').pop() ?? filename

            return (
              <li
                key={filename}
                className="group flex cursor-pointer items-center justify-between gap-2 rounded-md px-2 py-1 transition hover:bg-gray-100"
                onDoubleClick={() => insertMarkdown(assetUrl)}
              >
                <div className="flex flex-1 items-center gap-3">
                  {isImage ? (
                    <AssetPreviewTooltip url={assetUrl} name={baseName}>
                      <img
                        src={assetUrl}
                        alt={baseName}
                        className="h-10 w-10 rounded border object-cover"
                      />
                    </AssetPreviewTooltip>
                  ) : (
                    <AssetPreviewTooltip url={assetUrl} name={baseName}>
                      <div className="flex h-10 w-10 items-center justify-center rounded border bg-gray-100 text-gray-500">
                        <FileText size={18} />
                      </div>
                    </AssetPreviewTooltip>
                  )}
                  <span className="truncate text-sm text-gray-800 hover:underline">
                    {baseName}
                  </span>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-gray-400 hover:text-red-600"
                  onClick={() => handleDelete(baseName)}
                  title="Delete asset"
                >
                  <Trash2 size={16} />
                </Button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
