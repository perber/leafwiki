import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/stores/auth"
import { useUserStore } from "@/stores/users"
import { useState } from "react"

type Props = {
  userId: string
  username: string
}

export function DeleteUserButton({ userId, username }: Props) {
  const { user: currentUser } = useAuthStore()
  const { deleteUser } = useUserStore()

  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)

  const isSelf = currentUser?.id === userId

  const handleDelete = async () => {
    setLoading(true)
    try {
      await deleteUser(userId)
      setOpen(false)
    } catch (err) {
        console.error("Error deleting user:", err)
        // Optionally show an error message to the user
    } finally {
      setLoading(false)
    }
  }

  if (isSelf) return null

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button size="sm" variant="destructive">Delete</Button>
      </AlertDialogTrigger>

      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            Are you sure you want to delete this user?
          </AlertDialogTitle>
        </AlertDialogHeader>

        <p className="text-sm text-gray-600">
          The user <strong>{username}</strong> will be permanently removed from the system.
        </p>

        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            disabled={loading}
          >
            {loading ? "Deleting..." : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
