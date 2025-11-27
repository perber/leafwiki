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
    <div className="flex flex-1 flex-col items-center">
      <button
        onClick={() => onEditClicked()}
        className="group relative flex items-center gap-1 text-base font-semibold text-gray-800 hover:underline"
      >
        <TooltipWrapper label={title} side="top" align="start">
          {title && (
            <span className="inline-block max-w-[15vw] truncate sm:max-w-[40vw]">
              {title}
            </span>
          )}
          <Pencil
            size={16}
            className="absolute top-1/2 -right-6 -translate-y-1/2 text-gray-400 transition-transform duration-200 ease-in-out group-hover:text-gray-600"
          />
          {dirty && !isMobile && (
            <span className="ml-2 text-xs text-yellow-600">(Changes)</span>
          )}

          {dirty && isMobile && (
            <span className="ml-2 text-xs text-yellow-600">*</span>
          )}
        </TooltipWrapper>
      </button>
      <span className="mt-1 inline-block max-w-[15vw] truncate rounded bg-gray-200 px-2 py-0.5 font-mono text-xs text-gray-700 sm:max-w-[40vw]">
        {slug}
      </span>
    </div>
  )
}
