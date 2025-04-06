import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useUserStore } from "@/stores/users"
import { useState } from "react"

type Props = {
  userId: string
  username: string
}

export function ChangePasswordDialog({ userId, username }: Props) {
  const [open, setOpen] = useState(false)
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const { users, updateUser } = useUserStore()
  const user = users.find(u => u.id === userId)

  if (!user) return null

  const handleChange = async () => {
    if (!password || password !== confirm) {
      setError("Passwords are not matching.")
      return
    }

    setLoading(true)
    try {
      await updateUser({
        ...user,
        password,
      })
      setOpen(false)
    } catch (err) {
      setError("Error updating password.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" variant="secondary">Change Password</Button>
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change password for user {username}</DialogTitle>
        </DialogHeader>

        <div className="space-y-3 pt-2">
          <Input
            type="password"
            placeholder="New Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <Input
            type="password"
            placeholder="Confirm Password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
          {error && <p className="text-red-500 text-sm">{error}</p>}
        </div>

        <DialogFooter className="pt-4">
          <Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
          <Button onClick={handleChange} disabled={loading}>
            {loading ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
