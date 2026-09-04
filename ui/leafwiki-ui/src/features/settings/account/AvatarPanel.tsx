import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { avatarUrl } from '@/lib/api/avatar'
import { mapApiError } from '@/lib/api/errors'
import { formatBytes } from '@/lib/config'
import { useAvatarStore } from '@/stores/avatar'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { TrashIcon, UploadIcon } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

export function AvatarPanel() {
  const { t } = useTranslation('settings')
  const user = useSessionStore((s) => s.user)
  const { avatarVersion, isLoading, uploadAvatar, deleteAvatar } =
    useAvatarStore()
  const maxAvatarUploadSizeBytes = useConfigStore(
    (s) => s.maxAvatarUploadSizeBytes,
  )
  const avatarAllowedExts = useConfigStore((s) => s.avatarAllowedExts)

  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return

    try {
      await uploadAvatar(file)
      toast.success(t('account.avatar.uploadSuccess'))
    } catch (err) {
      toast.error(mapApiError(err, t('account.avatar.uploadFailed')).message)
    }
  }

  const handleDelete = async () => {
    try {
      await deleteAvatar()
      toast.success(t('account.avatar.deleteSuccess'))
    } catch (err) {
      toast.error(mapApiError(err, t('account.avatar.deleteFailed')).message)
    }
  }

  if (!user) return null

  return (
    <div data-testid="account-avatar-panel">
      <div className="settings__preview">
        <span className="settings__preview-label">
          {t('account.avatar.currentLabel')}
        </span>
        <Avatar className="h-16 w-16">
          <AvatarImage
            src={avatarUrl(user.id, avatarVersion)}
            alt={user.username}
          />
          <AvatarFallback>{user.username[0].toUpperCase()}</AvatarFallback>
        </Avatar>
        <Button
          variant="destructive"
          size="sm"
          onClick={handleDelete}
          disabled={isLoading}
        >
          <TrashIcon />
        </Button>
      </div>

      <div className="settings__field">
        <Label>{t('account.avatar.uploadButton')}</Label>
        <input
          type="file"
          ref={fileInputRef}
          onChange={handleUpload}
          accept={avatarAllowedExts.join(',')}
          className="hidden"
          data-testid="account-avatar-file-input"
        />
        <Button
          variant="outline"
          onClick={() => fileInputRef.current?.click()}
          disabled={isLoading}
        >
          <UploadIcon className="mr-2 h-4 w-4" />
          {t('account.avatar.uploadButton')}
        </Button>
        <p className="settings__hint">
          {t('account.avatar.hint', {
            exts: avatarAllowedExts
              .map((ext) => ext.replace('.', '').toUpperCase())
              .join(', '),
            size: formatBytes(maxAvatarUploadSizeBytes),
          })}
        </p>
      </div>
    </div>
  )
}
