import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import i18next from '@/lib/i18n'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { PageNode } from '@/lib/api/pages'
import { DIALOG_WIKILINK_DISAMBIGUATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { File, FolderTree } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

type WikiLinkDisambiguationDialogProps = {
  title: string
}

export function WikiLinkDisambiguationDialog({
  title,
}: WikiLinkDisambiguationDialogProps) {
  const navigate = useNavigate()
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const isOpen = useDialogsStore(
    (s) => s.dialogType === DIALOG_WIKILINK_DISAMBIGUATION,
  )
  const matches: PageNode[] = useTreeStore((s) =>
    title ? s.getPagesByTitle(title) : [],
  )
  const openAncestorsForPath = useTreeStore((s) => s.openAncestorsForPath)

  const handleSelect = (path: string) => {
    openAncestorsForPath(path)
    navigate(`/${path}`, { state: createNavigationVisitState() })
    closeDialog()
  }

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) queueMicrotask(() => closeDialog())
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {i18next.t('wikiLinkDisambiguation.title', { ns: 'editor' })}
          </DialogTitle>
          <DialogDescription>
            {i18next.t('wikiLinkDisambiguation.description', {
              ns: 'editor',
              title,
            })}
          </DialogDescription>
        </DialogHeader>

        <ul className="space-y-1">
          {matches.map((page) => {
            const Icon = page.kind === 'section' ? FolderTree : File
            return (
              <li key={page.id}>
                <button
                  type="button"
                  onClick={() => handleSelect(page.path)}
                  className="hover:bg-accent flex w-full items-start gap-3 rounded-md px-3 py-2 text-left"
                >
                  <Icon className="mt-0.5 h-4 w-4 shrink-0" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {page.title}
                    </span>
                    <span className="text-muted-foreground block truncate text-xs">
                      /{page.path}
                    </span>
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      </DialogContent>
    </Dialog>
  )
}
