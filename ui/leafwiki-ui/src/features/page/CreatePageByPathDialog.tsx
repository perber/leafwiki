import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ensurePage, lookupPath, PathLookupResult } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDebounce } from '@/lib/useDebounce'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { Check, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

type CreatePageByPathDialogProps = {
  initialPath?: string
  readOnlyPath?: boolean
}

export function CreatePageByPathDialog({
  initialPath,
  readOnlyPath,
}: CreatePageByPathDialogProps) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === 'create-by-path')
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

  const handleCancel = () => {
    closeDialog()
  }

  const handleCreate = async (editAfterCreate: boolean) => {
    setLoading(true)
    setFieldErrors({})

    try {
      // Here you would call your API to create the page
      await ensurePage(path, title)
      await reloadTree()
      // On success, close the dialog
      if (editAfterCreate) {
        // strip leading /
        const cleanPath = path.startsWith('/') ? path.slice(1) : path
        navigate(`/e/${cleanPath}`)
      }
      closeDialog()
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      setLoading(false)
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
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          closeDialog()
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create a new page</DialogTitle>
          <DialogDescription>Please enter the title</DialogDescription>
        </DialogHeader>
        <div>
          {lookup?.exists && (
            <div className="rounded bg-red-100 p-4 text-sm text-red-800">
              A page already exists at this path.
            </div>
          )}
          {lookup && !lookup.exists && lookup.segments.length > 0 && (
            <>
              <strong className="text-small">Result of path lookup:</strong>
              <ul className="mt-2 h-24 list-inside list-none space-y-4 overflow-auto rounded-md bg-gray-100 p-1">
                {lookup.segments.map((segment, index) => (
                  <li
                    key={index}
                    className="mb-1 flex items-center gap-1 text-xs"
                  >
                    {segment.exists ? (
                      <Check className="text-green-600" size={12} />
                    ) : (
                      <X className="text-red-600" size={12} />
                    )}{' '}
                    <span className="font-mono">{segment.slug}</span>{' '}
                    {segment.exists ? 'exists' : 'will be created'}
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
        <div className="space-y-4">
          <FormInput
            autoFocus={true}
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
        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={handleCancel}
            onSave={async () => await handleCreate(true)}
            saveLabel={loading ? 'Creating…' : 'Create & Edit Page'}
            disabled={isCreateButtonDisabled}
            loading={loading}
          ></FormActions>
        </div>
      </DialogContent>
    </Dialog>
  )
}
