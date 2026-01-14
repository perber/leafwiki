import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useBrandingStore } from '@/stores/branding'
import {
  ImageIcon,
  Loader2,
  SaveIcon,
  TrashIcon,
  UploadIcon,
} from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

export default function BrandingSettings() {
  const {
    siteName,
    logoFile,
    faviconFile,
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

  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: 'Branding Settings' })

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
      toast.success('Logo deleted successfully')
    } catch {
      toast.error('Failed to delete logo')
    }
  }

  const handleFaviconDelete = async () => {
    try {
      await deleteFavicon()
      toast.success('Favicon deleted successfully')
    } catch {
      toast.error('Failed to delete favicon')
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await updateBranding({
        siteName: localSiteName,
      })
      toast.success('Branding settings saved')
    } catch {
      toast.error('Failed to save branding settings')
    } finally {
      setSaving(false)
    }
  }

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      await uploadLogo(file)
      toast.success('Logo uploaded successfully')
    } catch {
      toast.error('Failed to upload logo')
    }
  }

  const handleFaviconUpload = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      await uploadFavicon(file)
      toast.success('Favicon uploaded successfully')
    } catch {
      toast.error('Failed to upload favicon')
    }
  }

  return (
    <>
      <div className="branding-settings">
        <h1 className="branding-settings__title">Branding Settings</h1>
        <div className="branding-settings__section">
          <h2 className="branding-settings__section-title">Site Name</h2>
          <p className="branding-settings__section-description">
            The name displayed in the header, page titles, and login screen.
          </p>
          <div className="branding-settings__field">
            <Label htmlFor="siteName">Site Name</Label>
            <Input
              id="siteName"
              value={localSiteName}
              onChange={(e) => setLocalSiteName(e.target.value)}
              placeholder="LeafWiki"
            />
          </div>
        </div>

        <div className="branding-settings__section">
          <h2 className="branding-settings__section-title">Logo</h2>
          <p className="branding-settings__section-description">
            The logo displayed in the header next to the site name.
          </p>

          <div className="branding-settings__preview">
            <span className="branding-settings__preview-label">
              Current Logo:
            </span>
            {logoFile ? (
              <>
                <img
                  src={`/branding/${logoFile}`}
                  alt="Logo"
                  className="branding-settings__preview-image"
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
              <span className="branding-settings__preview-placeholder">
                No logo uploaded
              </span>
            )}
          </div>

          <div className="branding-settings__field">
            <Label>Upload Logo</Label>{' '}
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
              Upload Image
            </Button>
            <p className="branding-settings__hint">
              Accepts {logoExts.map((ext) => ext.toUpperCase()).join(', ')}, max
              size {(maxLogoSize / (1024 * 1024)).toFixed(1)} MB
            </p>
          </div>
        </div>

        <div className="branding-settings__section">
          <h2 className="branding-settings__section-title">Favicon</h2>
          <p className="branding-settings__section-description">
            The icon displayed in the browser tab.
          </p>

          <div className="branding-settings__preview">
            <span className="branding-settings__preview-label">
              Current Favicon:
            </span>
            {faviconFile ? (
              <>
                {' '}
                <img
                  src={`/branding/${faviconFile}`}
                  alt="Favicon"
                  className="branding-settings__preview-favicon"
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
              <span className="branding-settings__preview-placeholder">
                Using default favicon
              </span>
            )}
          </div>

          <div className="branding-settings__field">
            <Label>Upload Favicon</Label>
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
              Upload Favicon
            </Button>
            <p className="branding-settings__hint">
              Accepts {faviconExts.map((ext) => ext.toUpperCase()).join(', ')},
              max size {(maxFaviconSize / (1024 * 1024)).toFixed(1)} MB
            </p>
          </div>
        </div>

        <div className="branding-settings__actions">
          <Button onClick={handleSave} disabled={saving || isLoading}>
            {saving ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <SaveIcon className="mr-2 h-4 w-4" />
            )}
            Save Settings
          </Button>
        </div>
      </div>
    </>
  )
}
