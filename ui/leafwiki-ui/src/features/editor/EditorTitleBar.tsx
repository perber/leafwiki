import { DIALOG_EDIT_PAGE_METADATA } from '@/lib/registries'
import { useAppMode } from '@/lib/useAppMode'
import { useIsMobile } from '@/lib/useIsMobile'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { Pencil } from 'lucide-react'
import { TooltipWrapper } from '../../components/TooltipWrapper'
import { usePageEditorStore } from './pageEditor'

export function EditorTitleBar() {
  const isMobile = useIsMobile()
  const appMode = useAppMode()
  const page = usePageEditorStore((state) => state.page)
  const title = usePageEditorStore((state) => state.title)
  const slug = usePageEditorStore((state) => state.slug)
  const setTitle = usePageEditorStore((state) => state.setTitle)
  const setSlug = usePageEditorStore((state) => state.setSlug)
  const openDialog = useDialogsStore((s) => s.openDialog)
  const getPageByPath = useTreeStore((state) => state.getPageByPath)
  const dirty = usePageEditorStore((s) => {
    const { page, title, slug, content } = s
    if (!page) return false
    return (
      page.title !== title || page.slug !== slug || page.content !== content
    )
  })

  const onEditClicked = () => {
    if (!page) return

    const parentId = () => {
      const parentPath = page.path.split('/').slice(0, -1).join('/')
      const p = getPageByPath(parentPath)
      if (!p) return ''
      return p.id
    }

    openDialog(DIALOG_EDIT_PAGE_METADATA, {
      title: title,
      currentId: page.id,
      slug: slug,
      parentId: parentId(),
      onChange: (title: string, slug: string) => {
        setTitle(title)
        setSlug(slug)
      },
    })
  }

  if (appMode !== 'edit') {
    return null
  }

  if (page == null) {
    return null
  }

  return (
    <div className="editor-title-bar">
      <button onClick={onEditClicked} className="editor-title-bar__button">
        <TooltipWrapper label={title} side="top" align="start">
          {title && (
            <span className="editor-title-bar__title">
              {title}
            </span>
          )}
          <Pencil size={16} className="editor-title-bar__icon" />
          {dirty && !isMobile && (
            <span className="editor-title-bar__dirty-indicator">
              (Changes)
            </span>
          )}

          {dirty && isMobile && (
            <span className="editor-title-bar__dirty-indicator">
              *
            </span>
          )}
        </TooltipWrapper>
      </button>
      <span className="editor-title-bar__slug">{slug}</span>
    </div>
  )
}
