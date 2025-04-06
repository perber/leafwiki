import { Button } from '@/components/ui/button'
import { deleteAsset, getAssets, uploadAsset } from '@/lib/api'
import { Trash2, UploadCloud } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

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
    try {
      const res = await uploadAsset(pageId, file)
      await loadAssets()

      if (onInsert) {
        const markdown = generateMarkdownLink(file.name.split('.')[0], res.file)
        onInsert(`${markdown}\n`)
      }
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
                className="group flex items-center justify-between gap-2 text-gray-700"
              >
                <div
                  className="flex cursor-pointer items-center gap-2"
                  onClick={() => insertMarkdown(assetUrl)}
                >
                  {isImage && (
                    <img
                      src={assetUrl}
                      alt={baseName}
                      className="h-8 w-8 rounded object-cover"
                    />
                  )}
                  <span
                    title="Click to insert Markdown"
                    className="max-w-[120px] truncate hover:underline"
                  >
                    {baseName}
                  </span>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-red-500 hover:text-red-700"
                  onClick={() => handleDelete(baseName)}
                >
                  <Trash2 size={14} />
                </Button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
