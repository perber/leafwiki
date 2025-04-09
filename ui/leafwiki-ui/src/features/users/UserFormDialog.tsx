import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { User } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

type Props = {
  user?: User
}

export function UserFormDialog({ user }: Props) {
  const isEdit = !!user
  const [open, setOpen] = useState(false)

  const [username, setUsername] = useState(user?.username || '')
  const [email, setEmail] = useState(user?.email || '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'editor'>(user?.role || 'editor')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { createUser, updateUser } = useUserStore()
  const { user: currentUser } = useAuthStore()

  const handleSubmit = async () => {
    if (!username || !email || (!isEdit && !password)) return

    const userData = {
      id: user?.id || '',
      username,
      email,
      password,
      role,
    }

    setLoading(true)

    try {
      if (isEdit) {
        await updateUser({ ...userData, password: password || undefined })
      } else {
        await createUser(userData)
      }
      resetForm()
      setOpen(false)
      toast.success('User saved successfully')
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error saving user')
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setOpen(false)
    resetForm()
  }

  const resetForm = () => {
    setUsername('')
    setEmail('')
    setPassword('')
    setRole('editor')
    setFieldErrors({})
  }

  const isOwnUser = user?.id === currentUser?.id

  useEffect(() => {
    if (open && !isEdit) {
      setUsername('')
      setEmail('')
      setPassword('')
      setRole('editor')
    }
  }, [open])


  return (
    <Dialog open={open}
      onOpenChange={(isOpen) => {
        setOpen(isOpen)
        if (!isOpen) resetForm()
      }}
    >
      <DialogTrigger asChild>
        {isEdit ? (
          <Button size="sm" variant="outline">
            Edit User
          </Button>
        ) : (
          <Button variant="default">New User</Button>
        )}
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit User' : 'New User'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <FormInput
            autoFocus={true}
            label="username"
            value={username}
            onChange={(val) => {
              setUsername(val)
              setFieldErrors((prev) => ({ ...prev, username: '' }))
            }}
            placeholder="username"
            error={fieldErrors.username}
          />
          <FormInput
            label="email"
            value={email}
            onChange={(val) => {
              setEmail(val)
              setFieldErrors((prev) => ({ ...prev, email: '' }))
            }}
            placeholder="email"
            error={fieldErrors.email}
          />
          {!isEdit && (
            <FormInput
              label="password"
              value={password}
              onChange={(val) => {
                setPassword(val)
                setFieldErrors((prev) => ({ ...prev, password: '' }))
              }}
              placeholder="password"
              error={fieldErrors.password}
              type="password"
            />)}

          <select
            className={`w-full rounded-md border border-gray-300 px-3 py-2 text-sm ${fieldErrors.role ? 'border-red-500' : ''}`}
            value={role}
            onChange={(e) => {
              setRole(e.target.value as 'admin' | 'editor')
              setFieldErrors((prev) => ({ ...prev, role: '' }))
            }}
            disabled={isOwnUser}
          >
            <option value="editor">Editor</option>
            <option value="admin">Admin</option>
          </select>
          {fieldErrors.role && (
            <p className="text-sm text-red-500 mt-1">{fieldErrors.role}</p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <FormActions
              onCancel={handleCancel}
              onSave={handleSubmit}
              saveLabel={loading ? 'Saving...' : 'Save'}
              disabled={loading || !username || !email || (!isEdit && !password)}
              loading={loading}
            />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
