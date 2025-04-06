import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { User } from "@/lib/api"
import { useAuthStore } from "@/stores/auth"
import { useUserStore } from "@/stores/users"
import { useEffect, useState } from "react"

type Props = {
  user?: User
}

export function UserFormDialog({ user }: Props) {
  const isEdit = !!user
  const [open, setOpen] = useState(false)

  const [username, setUsername] = useState(user?.username || "")
  const [email, setEmail] = useState(user?.email || "")
  const [password, setPassword] = useState("")
  const [role, setRole] = useState<"admin" | "editor">(user?.role || "editor")

  const { createUser, updateUser } = useUserStore()
  const { user: currentUser } = useAuthStore()

  const handleSubmit = async () => {
    if (!username || !email || (!isEdit && !password)) return

    const userData = {
      id: user?.id || "",
      username,
      email,
      password,
      role,
    }

    try {
      if (isEdit) {
        await updateUser({ ...userData, password: password || undefined })
      } else {
        await createUser(userData)
      }
      setOpen(false)
    } catch (err) {
      console.error("Fehler beim Speichern:", err)
    }
  }

  const isOwnUser = user?.id === currentUser?.id

  useEffect(() => {
    if (open && !isEdit) {
      setUsername("")
      setEmail("")
      setPassword("")
      setRole("editor")
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {isEdit ? (
          <Button size="sm" variant="outline">Edit User</Button>
        ) : (
          <Button variant="default">New User</Button>
        )}
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit User" : "New User"}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <Input
            placeholder="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
          <Input
            placeholder="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          {!isEdit && (
            <Input
              placeholder="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          )}
          <select
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm"
            value={role}
            onChange={(e) => setRole(e.target.value as "admin" | "editor")}
            disabled={isOwnUser} // nicht den eigenen Admin-Status wegnehmen
          >
            <option value="editor">Editor</option>
            <option value="admin">Admin</option>
          </select>

          <div className="pt-2 flex justify-end gap-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSubmit} disabled={!username || !email || (!isEdit && !password)}>
              {isEdit ? "Save" : "Create"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
