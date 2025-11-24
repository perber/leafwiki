import { Button } from '@/components/ui/button'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'
import { useLocation } from 'react-router-dom'

export default function Page404() {
  const user = useAuthStore((s) => s.user)
  const readOnlyMode = useIsReadOnly()
  const appMode = useAppMode()
  const { pathname } = useLocation()
  const openDialog = useDialogsStore((s) => s.openDialog)

  return (
    <div>
      <h1 className="text-1xl mb-2 font-bold text-red-500">Page Not Found</h1>
      <p className="text-sm text-gray-500">
        The page you are looking for does not exist.
      </p>
      {(user || !readOnlyMode || appMode !== 'view') && (
        <>
          <p className="text-sm text-gray-500">
            Create the page by clicking the button below.
          </p>
          <Button
            className="mt-4"
            onClick={() =>
              openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
                initialPath: pathname,
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
