import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { User } from '@/lib/api/users'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_USER_FORM } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import { useUserStore } from '@/stores/users'
import { useState } from 'react'
import { toast } from 'sonner'

type UserFormDialogProps = {
  user?: User
}

export function UserFormDialog({ user }: UserFormDialogProps) {
  const isEdit = !!user
  const [username, setUsername] = useState(user?.username || '')
  const [email, setEmail] = useState(user?.email || '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'editor'>(user?.role || 'editor')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { createUser, updateUser } = useUserStore()
  const { user: currentUser } = useSessionStore()
  const isOwnUser = user?.id === currentUser?.id

  const handleSubmit = async (): Promise<boolean> => {
    if (!username || !email || (!isEdit && !password)) return false // Should not happen due to button disabling

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
      toast.success('User saved successfully')
      return true // Close the dialog
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error saving user')
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_USER_FORM}
      dialogTitle={isEdit ? 'Edit User' : 'New User'}
      dialogDescription={isEdit ? 'Edit user details' : 'Create a new user'}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleSubmit()
      }}
      testidPrefix="user-form-dialog"
      cancelButton={{ label: 'Cancel', variant: 'outline', disabled: loading }}
      buttons={[
        {
          label: 'Save',
          actionType: 'confirm',
          loading,
          disabled: loading || !username || !email || (!isEdit && !password),
        },
      ]}
    >
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
          />
        )}
        <Select
          disabled={isOwnUser}
          value={role}
          onValueChange={(role) => {
            setRole(role as 'admin' | 'editor')
            setFieldErrors((prev) => ({ ...prev, role: '' }))
          }}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select a role" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key="editor" value="editor">
              Editor
            </SelectItem>
            <SelectItem key="admin" value="admin">
              Admin
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </BaseDialog>
  )
}
