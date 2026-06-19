import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { mapApiError } from '@/lib/api/errors'
import { withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import {
  ImageIcon,
  Loader2,
  SaveIcon,
  TrashIcon,
  UploadIcon,
} from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

export default function BrandingSettings() {
  const {
    siteName,
    logoFile,
    faviconFile,
    logoVersion,
    faviconVersion,
    isLoading,
    logoExts,
    maxLogoSize,
    faviconExts,
    maxFaviconSize,
    loadBranding,
    updateBranding,
    uploadLogo,
    uploadFavicon,
    deleteLogo,
    deleteFavicon,
  } = useBrandingStore()

  const { t } = useTranslation('branding')

  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: t('pageTitle') })

  const [localSiteName, setLocalSiteName] = useState(siteName)
  const [saving, setSaving] = useState(false)

  const logoInputRef = useRef<HTMLInputElement>(null)
  const faviconInputRef = useRef<HTMLInputElement>(null)

  useLayoutEffect(() => {
    loadBranding()
  }, [loadBranding])

  useEffect(() => {
    setLocalSiteName(siteName)
  }, [siteName])

  const handleLogoDelete = async () => {
    try {
      await deleteLogo()
      toast.success(t('toast.logoDeleted'))
    } catch (err) {
      toast.error(mapApiError(err, t('toast.logoDeleteFailed')).message)
    }
  }

  const handleFaviconDelete = async () => {
    try {
      await deleteFavicon()
      toast.success(t('toast.faviconDeleted'))
    } catch (err) {
      toast.error(mapApiError(err, t('toast.faviconDeleteFailed')).message)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await updateBranding({
        siteName: localSiteName,
      })
      toast.success(t('toast.brandingSaved'))
    } catch (err) {
      toast.error(mapApiError(err, t('toast.brandingSaveFailed')).message)
    } finally {
      setSaving(false)
    }
  }

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      await uploadLogo(file)
      toast.success(t('toast.logoUploaded'))
    } catch (err) {
      toast.error(mapApiError(err, t('toast.logoUploadFailed')).message)
    }
  }

  const handleFaviconUpload = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      await uploadFavicon(file)
      toast.success(t('toast.faviconUploaded'))
    } catch (err) {
      toast.error(mapApiError(err, t('toast.faviconUploadFailed')).message)
    }
  }

  return (
    <>
      <div className="settings">
        <h1 className="settings__title">{t('pageTitle')}</h1>
        <div className="settings__section">
          <h2 className="settings__section-title">{t('siteName.title')}</h2>
          <p className="settings__section-description">
            {t('siteName.description')}
          </p>
          <div className="settings__field">
            <Label htmlFor="siteName">{t('siteName.label')}</Label>
            <Input
              id="siteName"
              value={localSiteName}
              onChange={(e) => setLocalSiteName(e.target.value)}
              placeholder={t('siteName.placeholder')}
            />
          </div>
        </div>

        <div className="settings__section">
          <h2 className="settings__section-title">{t('logo.title')}</h2>
          <p className="settings__section-description">
            {t('logo.description')}
          </p>

          <div className="settings__preview">
            <span className="settings__preview-label">
              {t('logo.currentLabel')}
            </span>
            {logoFile ? (
              <>
                <img
                  src={`${withBasePath(`/branding/${logoFile}`)}?v=${logoVersion}`}
                  alt={t('logo.title')}
                  className="settings__preview-image"
                />
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleLogoDelete}
                  disabled={isLoading}
                >
                  <TrashIcon />
                </Button>
              </>
            ) : (
              <span className="settings__preview-placeholder">
                {t('logo.noLogo')}
              </span>
            )}
          </div>

          <div className="settings__field">
            <Label>{t('logo.uploadLabel')}</Label>{' '}
            <input
              type="file"
              ref={logoInputRef}
              onChange={handleLogoUpload}
              accept={logoExts.map((ext) => `${ext}`).join(',')}
              className="hidden"
            />
            <Button
              variant="outline"
              onClick={() => logoInputRef.current?.click()}
              disabled={isLoading}
            >
              <UploadIcon className="mr-2 h-4 w-4" />
              {t('logo.uploadButton')}
            </Button>
            <p className="settings__hint">
              {t('logo.hint', {
                exts: logoExts.map((ext) => ext.toUpperCase()).join(', '),
                size: (maxLogoSize / (1024 * 1024)).toFixed(1),
              })}
            </p>
          </div>
        </div>

        <div className="settings__section">
          <h2 className="settings__section-title">{t('favicon.title')}</h2>
          <p className="settings__section-description">
            {t('favicon.description')}
          </p>

          <div className="settings__preview">
            <span className="settings__preview-label">
              {t('favicon.currentLabel')}
            </span>
            {faviconFile ? (
              <>
                {' '}
                <img
                  src={`${withBasePath(`/branding/${faviconFile}`)}?v=${faviconVersion}`}
                  alt={t('favicon.title')}
                  className="settings__preview-favicon"
                />
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleFaviconDelete}
                  disabled={isLoading}
                >
                  <TrashIcon />
                </Button>
              </>
            ) : (
              <span className="settings__preview-placeholder">
                {t('favicon.noFavicon')}
              </span>
            )}
          </div>

          <div className="settings__field">
            <Label>{t('favicon.uploadLabel')}</Label>
            <input
              type="file"
              ref={faviconInputRef}
              onChange={handleFaviconUpload}
              accept={faviconExts.map((ext) => `${ext}`).join(',')}
              className="hidden"
            />
            <Button
              variant="outline"
              onClick={() => faviconInputRef.current?.click()}
              disabled={isLoading}
            >
              <ImageIcon className="mr-2 h-4 w-4" />
              {t('favicon.uploadButton')}
            </Button>
            <p className="settings__hint">
              {t('favicon.hint', {
                exts: faviconExts.map((ext) => ext.toUpperCase()).join(', '),
                size: (maxFaviconSize / (1024 * 1024)).toFixed(1),
              })}
            </p>
          </div>
        </div>

        <div className="settings__actions">
          <Button onClick={handleSave} disabled={saving || isLoading}>
            {saving ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <SaveIcon className="mr-2 h-4 w-4" />
            )}
            {t('save')}
          </Button>
        </div>
      </div>
    </>
  )
}
