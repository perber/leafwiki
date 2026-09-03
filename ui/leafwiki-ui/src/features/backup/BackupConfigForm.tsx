import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { BackupConfigInput } from '@/lib/api/backup'
import { mapApiError } from '@/lib/api/errors'
import { DEFAULT_INTERVAL_MINUTES, useBackupStore } from '@/stores/backup'
import { CheckCircle2, Loader2, TriangleAlert } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type FormState = {
  remoteUrl: string
  branch: string
  authorName: string
  authorEmail: string
  sshKey: string
  sshKeyPath: string
  sshKnownHostsPath: string
  httpUsername: string
  httpPassword: string
  intervalMinutes: string
}

function emptyForm(): FormState {
  return {
    remoteUrl: '',
    branch: 'main',
    authorName: '',
    authorEmail: '',
    sshKey: '',
    sshKeyPath: '',
    sshKnownHostsPath: '',
    httpUsername: '',
    httpPassword: '',
    intervalMinutes: String(DEFAULT_INTERVAL_MINUTES),
  }
}

// The transport (and therefore which credential fields are shown) is derived
// from the remote URL scheme — the single source of truth the backend also
// uses. There is deliberately no separate "auth mode" selector to fall out of
// sync with it.
function remoteIsHttp(remoteUrl: string): boolean {
  const l = remoteUrl.trim().toLowerCase()
  return l.startsWith('http://') || l.startsWith('https://')
}

// BackupConfigForm is the settings-managed git backup configuration form:
// remote URL, credentials (SSH key or HTTPS username/token, chosen by the URL
// scheme), branch, commit author and sync interval, plus Test / Save / Disable.
export default function BackupConfigForm() {
  const { t } = useTranslation('backup')
  const {
    enabled,
    config,
    configLoading,
    configError,
    encryptionKeyAvailable,
    minIntervalMinutes,
    maxIntervalMinutes,
    testConfig,
    saveConfig,
    disable,
  } = useBackupStore()

  const [form, setForm] = useState<FormState>(emptyForm)
  const [hasSshKey, setHasSshKey] = useState(false)
  const [hasHttpPassword, setHasHttpPassword] = useState(false)
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  const [disabling, setDisabling] = useState(false)
  const [testResult, setTestResult] = useState<
    { ok: true } | { ok: false; message: string } | null
  >(null)

  // Seed the form from the loaded config once it arrives.
  useEffect(() => {
    if (!config) return
    setForm({
      remoteUrl: config.remoteUrl || '',
      branch: config.branch || 'main',
      authorName: config.authorName || '',
      authorEmail: config.authorEmail || '',
      sshKey: '',
      sshKeyPath: config.sshKeyPath || '',
      sshKnownHostsPath: config.sshKnownHostsPath || '',
      httpUsername: config.httpUsername || '',
      httpPassword: '',
      intervalMinutes: String(
        config.intervalMinutes || DEFAULT_INTERVAL_MINUTES,
      ),
    })
    setHasSshKey(config.hasSshKey)
    setHasHttpPassword(config.hasHttpPassword)
  }, [config])

  const intervalError = useMemo(() => {
    const n = Number(form.intervalMinutes)
    if (!Number.isFinite(n) || !Number.isInteger(n)) {
      return t('config.intervalInvalid')
    }
    if (n < minIntervalMinutes || n > maxIntervalMinutes) {
      return t('config.intervalOutOfRange', {
        min: minIntervalMinutes,
        max: maxIntervalMinutes,
      })
    }
    return ''
  }, [form.intervalMinutes, minIntervalMinutes, maxIntervalMinutes, t])

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
    setTestResult(null)
  }

  const buildInput = (): BackupConfigInput => ({
    remoteUrl: form.remoteUrl.trim(),
    branch: form.branch.trim(),
    authorName: form.authorName.trim(),
    authorEmail: form.authorEmail.trim(),
    sshKey: form.sshKey,
    sshKeyPath: form.sshKeyPath.trim(),
    sshKnownHostsPath: form.sshKnownHostsPath.trim(),
    httpUsername: form.httpUsername.trim(),
    httpPassword: form.httpPassword,
    intervalMinutes: Number(form.intervalMinutes),
  })

  const canSubmit = form.remoteUrl.trim() !== '' && intervalError === ''

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      await testConfig(buildInput())
      setTestResult({ ok: true })
    } catch (err) {
      setTestResult({
        ok: false,
        message: mapApiError(err, t('config.testFailed')).message,
      })
    } finally {
      setTesting(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await saveConfig(buildInput())
      toast.success(t('config.saveSuccess'))
      setForm((f) => ({ ...f, sshKey: '', httpPassword: '' }))
      setTestResult(null)
    } catch (err) {
      toast.error(mapApiError(err, t('config.saveFailed')).message)
    } finally {
      setSaving(false)
    }
  }

  const handleDisable = async () => {
    setDisabling(true)
    try {
      await disable()
      toast.success(t('config.disableSuccess'))
    } catch (err) {
      toast.error(mapApiError(err, t('config.disableFailed')).message)
    } finally {
      setDisabling(false)
    }
  }

  const showSsh = !remoteIsHttp(form.remoteUrl)

  return (
    <div className="settings__section">
      <h2 className="settings__section-title">{t('config.sectionTitle')}</h2>
      <p className="settings__section-description">
        {t('config.sectionDescription')}
      </p>

      {configLoading && (
        <div className="text-muted flex items-center gap-2 text-sm">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t('loading')}
        </div>
      )}

      {configError && <p className="text-error text-sm">{configError}</p>}

      {!encryptionKeyAvailable && (
        <p className="settings__hint">
          {t('config.credentialsUnencryptedHint')}
        </p>
      )}

      {!configLoading && (
        <div className="flex max-w-xl flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="backup-remote">{t('config.remoteUrl')}</Label>
            <Input
              id="backup-remote"
              value={form.remoteUrl}
              placeholder={t('config.remoteUrlPlaceholder')}
              onChange={(e) => set('remoteUrl', e.target.value)}
            />
          </div>

          {showSsh ? (
            <>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-ssh-key">{t('config.sshKey')}</Label>
                <Textarea
                  id="backup-ssh-key"
                  className="font-mono text-xs"
                  rows={4}
                  value={form.sshKey}
                  placeholder={
                    hasSshKey
                      ? t('config.secretKeepPlaceholder')
                      : t('config.sshKeyPlaceholder')
                  }
                  onChange={(e) => set('sshKey', e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-ssh-key-path">
                  {t('config.sshKeyPath')}
                </Label>
                <Input
                  id="backup-ssh-key-path"
                  value={form.sshKeyPath}
                  placeholder={t('config.sshKeyPathPlaceholder')}
                  onChange={(e) => set('sshKeyPath', e.target.value)}
                />
                <p className="text-muted text-xs">
                  {t('config.sshKeyPathHint')}
                </p>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-known-hosts">
                  {t('config.knownHostsPath')}
                </Label>
                <Input
                  id="backup-known-hosts"
                  value={form.sshKnownHostsPath}
                  placeholder={t('config.knownHostsPlaceholder')}
                  onChange={(e) => set('sshKnownHostsPath', e.target.value)}
                />
              </div>
            </>
          ) : (
            <>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-http-user">
                  {t('config.httpUsername')}
                </Label>
                <Input
                  id="backup-http-user"
                  value={form.httpUsername}
                  onChange={(e) => set('httpUsername', e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-http-pass">
                  {t('config.httpPassword')}
                </Label>
                <Input
                  id="backup-http-pass"
                  type="password"
                  value={form.httpPassword}
                  placeholder={
                    hasHttpPassword
                      ? t('config.secretKeepPlaceholder')
                      : t('config.httpPasswordPlaceholder')
                  }
                  onChange={(e) => set('httpPassword', e.target.value)}
                />
              </div>
            </>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="backup-branch">{t('config.branch')}</Label>
            <Input
              id="backup-branch"
              value={form.branch}
              placeholder={t('config.branchPlaceholder')}
              onChange={(e) => set('branch', e.target.value)}
            />
          </div>

          <div className="flex gap-3">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="backup-author-name">
                {t('config.authorName')}
              </Label>
              <Input
                id="backup-author-name"
                value={form.authorName}
                placeholder={t('config.authorNamePlaceholder')}
                onChange={(e) => set('authorName', e.target.value)}
              />
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="backup-author-email">
                {t('config.authorEmail')}
              </Label>
              <Input
                id="backup-author-email"
                value={form.authorEmail}
                placeholder={t('config.authorEmailPlaceholder')}
                onChange={(e) => set('authorEmail', e.target.value)}
              />
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="backup-interval">
              {t('config.intervalMinutes')}
            </Label>
            <Input
              id="backup-interval"
              type="number"
              min={minIntervalMinutes}
              max={maxIntervalMinutes}
              value={form.intervalMinutes}
              onChange={(e) => set('intervalMinutes', e.target.value)}
              className="max-w-32"
            />
            <p
              className={
                intervalError ? 'text-error text-xs' : 'text-muted text-xs'
              }
            >
              {intervalError ||
                t('config.intervalHint', {
                  min: minIntervalMinutes,
                  max: maxIntervalMinutes,
                })}
            </p>
          </div>

          {testResult && (
            <div
              className={
                testResult.ok
                  ? 'border-success/20 bg-success/5 flex items-center gap-2 rounded-md border p-2 text-sm'
                  : 'border-error/20 bg-error/5 flex items-center gap-2 rounded-md border p-2 text-sm'
              }
            >
              {testResult.ok ? (
                <>
                  <CheckCircle2 className="text-success h-4 w-4" />
                  <span className="text-success">
                    {t('config.testSuccess')}
                  </span>
                </>
              ) : (
                <>
                  <TriangleAlert className="text-error h-4 w-4" />
                  <span className="text-error">{testResult.message}</span>
                </>
              )}
            </div>
          )}

          <div className="settings__actions flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={handleTest}
              disabled={!canSubmit || testing || saving}
            >
              {testing && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('config.testButton')}
            </Button>
            <Button
              onClick={handleSave}
              disabled={!canSubmit || saving || testing}
            >
              {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('config.saveButton')}
            </Button>
            {enabled && (
              <Button
                variant="ghost"
                className="text-error hover:text-error"
                onClick={handleDisable}
                disabled={disabling}
              >
                {disabling && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('config.disableButton')}
              </Button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
