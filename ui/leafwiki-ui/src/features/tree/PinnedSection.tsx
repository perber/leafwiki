import { pinPage } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { Pin } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { PinnedPageItem } from './PinnedPageItem'

export function PinnedSection() {
  const { t } = useTranslation('viewer')
  const pinnedPages = useTreeStore((s) => s.pinnedPages)
  const reloadTree = useTreeStore((s) => s.reloadTree)

  if (pinnedPages.length === 0) return null

  const handleUnpin = async (id: string, version: string) => {
    try {
      await pinPage(id, version, false)
      await reloadTree()
      toast.success(t('pinned.unpinSuccess'))
    } catch {
      toast.error(t('pinned.pinError'))
    }
  }

  return (
    <div className="tree-view__pinned" data-testid="pinned-section">
      <div className="tree-view__pinned-header">
        <Pin size={11} />
        <span>{t('pinned.sectionTitle')}</span>
      </div>
      {pinnedPages.map((node) => (
        <PinnedPageItem
          key={node.id}
          node={node}
          onUnpin={() => handleUnpin(node.id, node.version)}
        />
      ))}
      <div className="tree-view__pinned-divider" />
    </div>
  )
}
