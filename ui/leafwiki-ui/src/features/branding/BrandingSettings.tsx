import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useBrandingStore } from '@/stores/branding'
import { ImageIcon, Loader2, SaveIcon, UploadIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

export default function BrandingSettings() {
    const {
        siteName,
        logoImagePath,
        faviconImagePath,
        isLoading,
        loadBranding,
        updateBranding,
        uploadLogo,
        uploadFavicon,
    } = useBrandingStore()

    const [localSiteName, setLocalSiteName] = useState(siteName)
    const [saving, setSaving] = useState(false)

    const logoInputRef = useRef<HTMLInputElement>(null)
    const faviconInputRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        loadBranding()
    }, [loadBranding])

    useEffect(() => {
        setLocalSiteName(siteName)
    }, [siteName])

    const handleSave = async () => {
        setSaving(true)
        try {
            await updateBranding({
                siteName: localSiteName,
            })
            toast.success('Branding settings saved')
        } catch (err) {
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
        } catch (err) {
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
        } catch (err) {
            toast.error('Failed to upload favicon')
        }
    }

    return (
        <>
            <title>Branding Settings - {siteName}</title>
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
                        {logoImagePath ? (
                            <img
                                src={`/branding/${logoImagePath}`}
                                alt="Logo"
                                className="branding-settings__preview-image"
                            />
                        ) : (
                            <span className="branding-settings__preview-placeholder">
                                No logo uploaded
                            </span>
                        )}
                    </div>

                    <div className="branding-settings__field">
                        <Label>Upload Logo</Label>
                        <input
                            type="file"
                            ref={logoInputRef}
                            onChange={handleLogoUpload}
                            accept=".png,.svg,.jpg,.jpeg,.webp"
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
                            Accepts PNG, SVG, JPG, JPEG, WebP
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
                        {faviconImagePath ? (
                            <img
                                src={`/branding/${faviconImagePath}`}
                                alt="Favicon"
                                className="branding-settings__preview-favicon"
                            />
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
                            accept=".png,.svg,.ico,.webp"
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
                            Accepts PNG, SVG, ICO, WebP
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
