import { FormActions } from '@/components/FormActions'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { deletePage } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useTreeStore } from '@/stores/tree'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export function DeletePageDialog({
  pageId,
  redirectUrl,
}: {
  pageId: string
  redirectUrl: string
}) {
  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const page = useTreeStore((s) => s.getPageById(pageId))

  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [deleteRecursive, setDeleteRecursive] = useState(false)
  const [_, setFieldErrors] = useState<Record<string, string>>({})

  if (!page) return null
  const hasChildren = page.children?.length > 0

  const handleDelete = async () => {
    setLoading(true)
    try {
      await deletePage(pageId, deleteRecursive)
      toast.success('Page deleted successfully')
      navigate(`/${redirectUrl}`)
      await reloadTree()
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error deleting page')
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setOpen(false)
    setLoading(false)
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (o === true) setDeleteRecursive(false)
      }}
    >
      <AlertDialogTrigger asChild>
        <Button variant="destructive" size="icon" className='h-8 w-8 rounded-full shadow-sm'>
          <Trash2 />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Page?</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogDescription>
          Are you sure you want to delete this page? This action cannot be
          undone.
        </AlertDialogDescription>

        {hasChildren && (
          <div className="space-y-1 text-sm text-gray-600">
            <label className="flex items-center gap-2">
              <Checkbox
                checked={deleteRecursive}
                onCheckedChange={(val) => setDeleteRecursive(!!val)}
              />
              Also delete all subpages
            </label>
          </div>
        )}

        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={handleCancel}
            onSave={handleDelete}
            saveLabel={loading ? 'Deleting...' : 'Delete'}
            disabled={loading}
            loading={loading}
          />
        </div>
      </AlertDialogContent>
    </AlertDialog>
  )
}
