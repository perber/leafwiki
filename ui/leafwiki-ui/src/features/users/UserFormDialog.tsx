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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type UserFormDialogProps = {
  user?: User
}

export function UserFormDialog({ user }: UserFormDialogProps) {
  const { t } = useTranslation('users')
  const isEdit = !!user
  const [username, setUsername] = useState(user?.username || '')
  const [email, setEmail] = useState(user?.email || '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'editor' | 'viewer'>(
    user?.role || 'editor',
  )
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { createUser, updateUser } = useUserStore()
  const { user: currentUser } = useSessionStore()
  const isOwnUser = user?.id === currentUser?.id

  const handleSubmit = async (): Promise<boolean> => {
    if (!username || !email || (!isEdit && !password)) return false

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
      toast.success(t('toast.saved'))
      return true
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, t('toast.saved'))
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_USER_FORM}
      dialogTitle={isEdit ? t('editUser') : t('newUser')}
      dialogDescription={
        isEdit ? t('editUserDescription') : t('newUserDescription')
      }
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleSubmit()
      }}
      testidPrefix="user-form-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
      }}
      buttons={[
        {
          label: t('actions.save'),
          actionType: 'confirm',
          loading,
          disabled: loading || !username || !email || (!isEdit && !password),
        },
      ]}
    >
      <div className="space-y-4 pt-2">
        <FormInput
          autoFocus={true}
          label={t('username')}
          name="username"
          value={username}
          onChange={(val) => {
            setUsername(val)
            setFieldErrors((prev) => ({ ...prev, username: '' }))
          }}
          placeholder={t('username')}
          autoComplete="username"
          error={fieldErrors.username}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        <FormInput
          label={t('email')}
          name="email"
          value={email}
          onChange={(val) => {
            setEmail(val)
            setFieldErrors((prev) => ({ ...prev, email: '' }))
          }}
          placeholder={t('email')}
          autoComplete="email"
          error={fieldErrors.email}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        {!isEdit && (
          <FormInput
            label={t('password')}
            name="new-password"
            value={password}
            onChange={(val) => {
              setPassword(val)
              setFieldErrors((prev) => ({ ...prev, password: '' }))
            }}
            placeholder={t('password')}
            autoComplete="new-password"
            error={fieldErrors.password}
            type="password"
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
        )}
        <Select
          disabled={isOwnUser}
          value={role}
          onValueChange={(nextRole) => {
            setRole(nextRole as 'admin' | 'editor' | 'viewer')
            setFieldErrors((prev) => ({ ...prev, role: '' }))
          }}
        >
          <SelectTrigger>
            <SelectValue placeholder={t('selectRole')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key="viewer" value="viewer">
              {t('roles.viewer')}
            </SelectItem>
            <SelectItem key="editor" value="editor">
              {t('roles.editor')}
            </SelectItem>
            <SelectItem key="admin" value="admin">
              {t('roles.admin')}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </BaseDialog>
  )
}
