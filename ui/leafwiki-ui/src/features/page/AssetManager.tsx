import { Button } from '@/components/ui/button'
import { deleteAsset, getAssets, uploadAsset } from '@/lib/api'
import { Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

type Props = {
  pageId: string
}

export function AssetManager({ pageId }: Props) {
  const [assets, setAssets] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const fileInput = useRef<HTMLInputElement>(null)

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

  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    try {
      await uploadAsset(pageId, file)
      await loadAssets()
    } catch (err) {
      console.error('Upload failed', err)
    } finally {
      if (fileInput.current) {
        fileInput.current.value = ''
      }
    }
  }

  const handleDelete = async (filename: string) => {
    try {
      await deleteAsset(pageId, filename)
      await loadAssets()
    } catch (err) {
      console.error('Delete failed', err)
    }
  }

  return (
    <div className="text-sm space-y-3">
      <div className="font-semibold text-gray-700">Assets</div>

      <input
        type="file"
        ref={fileInput}
        onChange={handleUpload}
        className="block w-full text-sm text-gray-600"
      />

      {loading ? (
        <p className="text-xs text-gray-500">Loading assets…</p>
      ) : assets.length === 0 ? (
        <p className="text-xs text-gray-400 italic">No assets yet</p>
      ) : (
        <ul className="space-y-2">
          {assets.map((filename) => {
            const assetUrl = filename
            const ext = filename.split('.').pop()?.toLowerCase()
            const isImage = ['png', 'jpg', 'jpeg', 'gif', 'webp'].includes(ext ?? '')
            const baseName = filename.split('/').pop() ?? filename            

            return (
              <li
                key={filename}
                className="group flex items-center justify-between gap-2 text-gray-700"
              >
                <div className="flex items-center gap-2">
                  {isImage && (
                    <img
                      src={assetUrl}
                      alt={baseName}
                      className="h-8 w-8 object-cover rounded"
                    />
                  )}
                  <a
                    href={assetUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:underline truncate max-w-[120px]"
                    title={baseName}
                  >
                    {baseName}
                  </a>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-red-500 hover:text-red-700"
                  onClick={() => handleDelete(filename)}
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