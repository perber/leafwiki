import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { ensurePage, lookupPath, PathLookupResult } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { buildEditUrl } from '@/lib/urlUtil'
import { useDebounce } from '@/lib/useDebounce'
import { useTreeStore } from '@/stores/tree'
import { Check, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

type CreatePageByPathDialogProps = {
  initialPath?: string
  readOnlyPath?: boolean
  forwardToEditMode?: boolean
}

export function CreatePageByPathDialog({
  initialPath,
  readOnlyPath,
  forwardToEditMode,
}: CreatePageByPathDialogProps) {
  // Dialog state from zustand store
  const navigate = useNavigate()

  // read the last segment from the initial path as title
  const initialTitle = initialPath?.split('/').pop() || 'unknown'

  const [title, setTitle] = useState(initialTitle)
  const [path, setPath] = useState(initialPath || '')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [lookup, setLookup] = useState<PathLookupResult | null>(null)
  const [loading, setLoading] = useState(false)
  const reloadTree = useTreeStore((s) => s.reloadTree)

  const debouncedPath = useDebounce(path, 300)

  const runLookup = async (path: string) => {
    try {
      const result = await lookupPath(path)
      if (result) {
        setLookup(result)
      }
    } catch (error) {
      console.error('Error looking up path:', error)
    }
  }

  const isCreateButtonDisabled = !title || !path || loading

  const handleCreate = async (): Promise<boolean> => {
    setLoading(true)
    setFieldErrors({})

    try {
      // Here you would call your API to create the page
      await ensurePage(path, title)
      await reloadTree()
      // On success, close the dialog
      if (forwardToEditMode) {
        navigate(buildEditUrl(path))
      }

      toast.success('Page created successfully')
      return true // Close the dialog
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  // Run lookup for initial path if it exists
  useEffect(() => {
    if (readOnlyPath && path) {
      // run lookup if the path exists!
      runLookup(path)
    }
  }, [path, readOnlyPath])

  // Run lookup when debounced path changes
  useEffect(() => {
    if (!readOnlyPath) {
      runLookup(debouncedPath)
    }
  }, [debouncedPath, readOnlyPath])

  const handleTitleChange = (val: string) => {
    setTitle(val)
  }

  return (
    <BaseDialog
      dialogTitle="Create a new page"
      dialogDescription="Please enter the title"
      dialogType={DIALOG_CREATE_PAGE_BY_PATH}
      testidPrefix="create-page-by-path-dialog"
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleCreate()
      }}
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading
            ? 'Creating...'
            : !forwardToEditMode
              ? 'Create'
              : 'Create & Edit',
          actionType: 'confirm',
          autoFocus: true,
          loading,
          disabled: isCreateButtonDisabled,
          variant: 'default',
        },
      ]}
    >
      <div>
        {lookup?.exists && (
          <div className="create-page-by-path-dialog__alert">
            A page already exists at this path.
          </div>
        )}
        {lookup && !lookup.exists && lookup.segments.length > 0 && (
          <>
            <strong className="create-page-by-path-dialog__lookup-title">
              Result of path lookup:
            </strong>
            <ul className="custom-scrollbar create-page-by-path-dialog__lookup-list">
              {lookup.segments.map((segment, index) => (
                <li
                  key={index}
                  className="create-page-by-path-dialog__lookup-item"
                >
                  {segment.exists ? (
                    <Check
                      className="create-page-by-path-dialog__lookup-item-icon--ok"
                      size={12}
                    />
                  ) : (
                    <X
                      className="create-page-by-path-dialog__lookup-item-icon--missing"
                      size={12}
                    />
                  )}{' '}
                  <span className="create-page-by-path-dialog__lookup-item-slug">
                    {segment.slug}
                  </span>{' '}
                  {segment.exists ? 'exists' : 'will be created'}
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
      <div className="page-dialog__fields">
        <FormInput
          autoFocus={true}
          testid="create-page-by-path-title-input"
          label="Title"
          value={title}
          onChange={(val) => {
            handleTitleChange(val)
            setFieldErrors((prev) => ({ ...prev, title: '' }))
          }}
          placeholder="Page title"
          error={fieldErrors.title}
        />
        <FormInput
          testid="create-page-by-path-path-input"
          label="Path"
          value={path}
          readOnly={readOnlyPath}
          onChange={(val) => {
            setPath(val)
            setFieldErrors((prev) => ({ ...prev, path: '' }))
          }}
          placeholder="Page path"
          error={fieldErrors.path}
        />
      </div>
    </BaseDialog>
  )
}
