import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { deletePage } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { Trash2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function DeletePageDialog({
  pageId,
  redirectUrl,
}: {
  pageId: string
  redirectUrl: string
}) {
  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)

  const handleDelete = async () => {
    await deletePage(pageId)
    await reloadTree()
    navigate(`/${redirectUrl}`)
  }

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button variant="destructive" size="sm">
          <Trash2 className="mr-1" />
          Delete
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Page?</AlertDialogTitle>
        </AlertDialogHeader>
        <p className="text-sm text-gray-600">
          Are you sure you want to delete this page? This action cannot be
          undone.
        </p>
        <div className="mt-4 flex justify-end">
          <Button variant="destructive" onClick={handleDelete}>
            Confirm Delete
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  )
}
