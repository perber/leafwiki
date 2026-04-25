import { Button } from '@/components/ui/button'
import { lookupPath } from '@/lib/api/pages'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { useEffect, useState } from 'react'

type Page404Props = {
  targetPath?: string
  allowCreate?: boolean
}

export default function Page404({
  targetPath,
  allowCreate = false,
}: Page404Props) {
  const user = useSessionStore((s) => s.user)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const readOnlyMode = useIsReadOnly()
  const openDialog = useDialogsStore((s) => s.openDialog)
  const [lookupState, setLookupState] = useState<{
    path: string | null
    canCreate: boolean
  }>({
    path: null,
    canCreate: false,
  })

  useEffect(() => {
    if (!allowCreate || !targetPath) return

    let active = true

    const loadLookup = async () => {
      try {
        const lookup = await lookupPath(targetPath)
        if (active) {
          setLookupState({
            path: targetPath,
            canCreate: lookup.canCreate && !lookup.exists,
          })
        }
      } catch {
        if (active) {
          setLookupState({
            path: targetPath,
            canCreate: false,
          })
        }
      }
    }

    void loadLookup()

    return () => {
      active = false
    }
  }, [allowCreate, targetPath])

  const showCreate =
    Boolean(targetPath) &&
    allowCreate &&
    lookupState.path === targetPath &&
    lookupState.canCreate &&
    (user || authDisabled) &&
    !readOnlyMode

  return (
    <div className="page404">
      <h1 className="page404__title">Page Not Found</h1>
      <p className="page404__text">
        The page you are looking for does not exist.
      </p>
      {showCreate && (
        <>
          <p className="page404__text">
            Create the page by clicking the button below.
          </p>
          <Button
            className="mt-4"
            data-testid="page404-create-page-button"
            onClick={() =>
              openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
                initialPath: targetPath,
                readOnlyPath: true,
                forwardToEditMode: true,
              })
            }
            variant={'outline'}
          >
            Create Page
          </Button>
        </>
      )}
    </div>
  )
}
