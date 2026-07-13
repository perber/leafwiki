import { pinPage } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { PinnedPageItem } from './PinnedPageItem'

export function PinnedSection() {
  const { t } = useTranslation('viewer')
  const pinnedPages = useTreeStore((s) => s.pinnedPages)
  const setPinnedLocally = useTreeStore((s) => s.setPinnedLocally)

  if (pinnedPages.length === 0) return null

  const handleUnpin = async (id: string, version: string) => {
    try {
      const updated = await pinPage(id, version, false)
      setPinnedLocally(id, false, updated.version)
      toast.success(t('pinned.unpinSuccess'))
    } catch {
      toast.error(t('pinned.pinError'))
    }
  }

  return (
    <div className="tree-view__pinned" data-testid="pinned-section">
      {pinnedPages.map((node) => (
        <PinnedPageItem
          key={node.id}
          node={node}
          onUnpin={() => handleUnpin(node.id, node.version)}
        />
      ))}
    </div>
  )
}
