import BaseDialog from '@/components/BaseDialog'
import { Checkbox } from '@/components/ui/checkbox'
import { fetchLinkStatus, type Backlink } from '@/lib/api/links'
import { asApiLocalizedError } from '@/lib/api/errors'
import { deletePage, NODE_KIND_PAGE } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useViewerStore } from '../viewer/viewer'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { AlertTriangle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export type DeletePageDialogProps = {
  pageId: string
  redirectTo: string
}

export function DeletePageDialog({
  pageId,
  redirectTo,
}: DeletePageDialogProps) {
  const enableLinkRefactor = useConfigStore((s) => s.enableLinkRefactor)
  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const page = useTreeStore((s) => s.getPageById(pageId))

  const [loading, setLoading] = useState(false)
  const [deleteRecursive, setDeleteRecursive] = useState(false)
  const [pageModifiedWarning, setPageModifiedWarning] = useState(false)
  const [backlinksLoading, setBacklinksLoading] = useState(false)
  const [backlinksError, setBacklinksError] = useState<string | null>(null)
  const [backlinks, setBacklinks] = useState<Backlink[]>([])
  const [, setFieldErrors] = useState<Record<string, string>>({})

  useEffect(() => {
    if (!enableLinkRefactor) {
      setBacklinksLoading(false)
      setBacklinksError(null)
      setBacklinks([])
      return
    }

    let cancelled = false

    const loadBacklinks = async () => {
      setBacklinksLoading(true)
      setBacklinksError(null)

      try {
        const status = await fetchLinkStatus(pageId)
        if (cancelled) return
        setBacklinks(status.backlinks ?? [])
      } catch (err) {
        if (cancelled) return
        const message =
          err instanceof Error ? err.message : 'Failed to load page references'
        setBacklinksError(message)
        setBacklinks([])
      } finally {
        if (!cancelled) {
          setBacklinksLoading(false)
        }
      }
    }

    void loadBacklinks()

    return () => {
      cancelled = true
    }
  }, [enableLinkRefactor, pageId])

  if (!page) return null
  const hasChildren = (page.children?.length ?? 0) > 0
  const itemLabel = page.kind === NODE_KIND_PAGE ? 'page' : 'section'
  const itemLabelCapitalized = page.kind === NODE_KIND_PAGE ? 'Page' : 'Section'

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deletePage(pageId, deleteRecursive, page?.version ?? '')
      toast.success(`${itemLabelCapitalized} deleted successfully`)
      navigate(redirectTo)
      reloadTree().catch(console.error)
      return true
    } catch (err) {
      console.warn(err)
      const localized = asApiLocalizedError(err)
      if (localized?.code === 'page_version_conflict') {
        reloadTree().catch(console.error)
        const viewerPage = useViewerStore.getState().page
        if (viewerPage?.id === pageId && viewerPage.path) {
          useViewerStore
            .getState()
            .loadPageData(viewerPage.path)
            .catch(console.error)
        }
        setPageModifiedWarning(true)
      } else {
        handleFieldErrors(err, setFieldErrors, `Error deleting ${itemLabel}`)
      }
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_PAGE_CONFIRMATION}
      dialogTitle={`Delete ${itemLabelCapitalized}?`}
      dialogDescription={`Are you sure you want to delete this ${itemLabel}? This action cannot be undone.`}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      defaultAction="cancel"
      testidPrefix="delete-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? 'Deleting...' : 'Delete',
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: loading,
          variant: 'destructive',
        },
      ]}
    >
      <div className="space-y-3">
        {pageModifiedWarning && (
          <div
            className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900"
            data-testid="delete-page-dialog-modified-warning"
          >
            <div className="flex items-start gap-2 font-medium">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>This page was modified by another user.</span>
            </div>
            <p className="mt-1">
              The page has been refreshed. Do you still want to delete it?
            </p>
          </div>
        )}
        {enableLinkRefactor &&
          (backlinksLoading ? (
            <p
              className="text-muted-foreground text-sm"
              data-testid="delete-page-dialog-backlinks-loading"
            >
              Checking which pages reference this page...
            </p>
          ) : backlinksError ? (
            <div
              className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900"
              data-testid="delete-page-dialog-backlinks-error"
            >
              Could not load page references. Deleting will still work, but link
              impact could not be shown.
            </div>
          ) : backlinks.length > 0 ? (
            <div
              className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950"
              data-testid="delete-page-dialog-backlinks-warning"
            >
              <div className="flex items-start gap-2 font-medium">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>
                  This page is referenced by {backlinks.length} page
                  {backlinks.length === 1 ? '' : 's'}.
                </span>
              </div>
              <p className="mt-2 text-sm">
                Deleting this page will leave those links broken.
              </p>
              <ul
                className="mt-3 max-h-40 space-y-1 overflow-auto pr-1 text-sm"
                data-testid="delete-page-dialog-backlinks-list"
              >
                {backlinks.map((backlink) => (
                  <li key={backlink.from_page_id}>
                    <Link className="underline" to={backlink.from_path}>
                      {backlink.from_title}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <p
              className="text-muted-foreground text-sm"
              data-testid="delete-page-dialog-no-backlinks"
            >
              No pages currently reference this page.
            </p>
          ))}

        {hasChildren && (
          <div className="delete-page-dialog__recursive">
            <label className="delete-page-dialog__recursive-label">
              <Checkbox
                data-testid="delete-page-dialog-recursive-delete-checkbox"
                checked={deleteRecursive}
                onCheckedChange={(val) => setDeleteRecursive(!!val)}
              />
              Also delete all subpages
            </label>
          </div>
        )}
      </div>
    </BaseDialog>
  )
}
