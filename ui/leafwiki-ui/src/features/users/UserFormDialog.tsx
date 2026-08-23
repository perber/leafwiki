import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { User } from '@/lib/api/users'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_USER_FORM } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
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
  const smtpEnabled = useConfigStore((s) => s.smtpEnabled)
  // The invite/password-now toggle only makes sense on create — editing an
  // existing user's password already has its own dedicated flow (change
  // password), and an invite can't be "resent" from this dialog.
  const [mode, setMode] = useState<'password' | 'invite'>('password')
  const isInvite = !isEdit && smtpEnabled && mode === 'invite'
  const [username, setUsername] = useState(user?.username || '')
  const [email, setEmail] = useState(user?.email || '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'editor' | 'viewer'>(
    user?.role || 'editor',
  )
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { createUser, updateUser, inviteUser } = useUserStore()
  const { user: currentUser } = useSessionStore()
  const isOwnUser = user?.id === currentUser?.id

  const handleSubmit = async (): Promise<boolean> => {
    if (!username || !email || (!isEdit && !isInvite && !password)) {
      return false // Should not happen due to button disabling
    }

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
        toast.success(t('userForm.successToast'))
      } else if (isInvite) {
        const { emailSent } = await inviteUser({ username, email, role })
        if (emailSent) {
          toast.success(t('userForm.inviteSuccessToast'))
        } else {
          toast.warning(t('invite.emailNotSentWarning'))
        }
      } else {
        await createUser(userData)
        toast.success(t('userForm.successToast'))
      }
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
          disabled:
            loading ||
            !username ||
            !email ||
            (!isEdit && !isInvite && !password),
        },
      ]}
    >
      <div className="space-y-4 pt-2">
        {!isEdit && smtpEnabled && (
          <Tabs value={mode} onValueChange={(v) => setMode(v as typeof mode)}>
            <TabsList className="w-full">
              <TabsTrigger value="password" className="flex-1">
                {t('userForm.modeSetPassword')}
              </TabsTrigger>
              <TabsTrigger value="invite" className="flex-1">
                {t('userForm.modeInvite')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        )}
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
        {!isEdit && !isInvite && (
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
