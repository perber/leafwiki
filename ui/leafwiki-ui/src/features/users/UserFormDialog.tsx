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
      toast.success(t('userForm.successToast'))
      return true // Close the dialog
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, t('userForm.errorFallback'))
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_USER_FORM}
      dialogTitle={isEdit ? t('userForm.editTitle') : t('userForm.newTitle')}
      dialogDescription={
        isEdit ? t('userForm.editDescription') : t('userForm.newDescription')
      }
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleSubmit()
      }}
      testidPrefix="user-form-dialog"
      cancelButton={{
        label: t('userForm.cancel'),
        variant: 'outline',
        disabled: loading,
      }}
      buttons={[
        {
          label: t('userForm.save'),
          actionType: 'confirm',
          loading,
          disabled: loading || !username || !email || (!isEdit && !password),
        },
      ]}
    >
      <div className="space-y-4 pt-2">
        <FormInput
          autoFocus={true}
          label={t('userForm.usernameLabel')}
          name="username"
          value={username}
          onChange={(val) => {
            setUsername(val)
            setFieldErrors((prev) => ({ ...prev, username: '' }))
          }}
          placeholder={t('userForm.usernamePlaceholder')}
          autoComplete="username"
          error={fieldErrors.username}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        <FormInput
          label={t('userForm.emailLabel')}
          name="email"
          value={email}
          onChange={(val) => {
            setEmail(val)
            setFieldErrors((prev) => ({ ...prev, email: '' }))
          }}
          placeholder={t('userForm.emailPlaceholder')}
          autoComplete="email"
          error={fieldErrors.email}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        {!isEdit && (
          <FormInput
            label={t('userForm.passwordLabel')}
            name="new-password"
            value={password}
            onChange={(val) => {
              setPassword(val)
              setFieldErrors((prev) => ({ ...prev, password: '' }))
            }}
            placeholder={t('userForm.passwordPlaceholder')}
            autoComplete="new-password"
            error={fieldErrors.password}
            type="password"
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
        )}
        <Select
          disabled={isOwnUser}
          value={role}
          onValueChange={(role) => {
            setRole(role as 'admin' | 'editor' | 'viewer')
            setFieldErrors((prev) => ({ ...prev, role: '' }))
          }}
        >
          <SelectTrigger>
            <SelectValue placeholder={t('userForm.rolePlaceholder')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key="viewer" value="viewer">
              {t('userForm.roleViewer')}
            </SelectItem>
            <SelectItem key="editor" value="editor">
              {t('userForm.roleEditor')}
            </SelectItem>
            <SelectItem key="admin" value="admin">
              {t('userForm.roleAdmin')}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </BaseDialog>
  )
}
