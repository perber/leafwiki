import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { mapApiError } from '@/lib/api/errors'
import type { BackupAuthMode, BackupConfigInput } from '@/lib/api/backup'
import { useDateTimeFormat } from '@/lib/useDateTimeFormat'
import { useBackupStore } from '@/stores/backup'
import {
  CheckCircle2,
  CloudUpload,
  GitMerge,
  Loader2,
  TriangleAlert,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'

const POLL_INTERVAL_MS = 5000

type FormState = {
  remoteUrl: string
  authMode: BackupAuthMode
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
    authMode: 'ssh',
    branch: 'main',
    authorName: '',
    authorEmail: '',
    sshKey: '',
    sshKeyPath: '',
    sshKnownHostsPath: '',
    httpUsername: '',
    httpPassword: '',
    intervalMinutes: '60',
  }
}

export default function BackupSettings() {
  const { t } = useTranslation('backup')
  const { formatDateTime } = useDateTimeFormat()
  const {
    enabled,
    envManaged,
    bootError,
    lastBackupAt,
    lastError,
    needsIntervention,
    conflictDetails,
    isLoading,
    isPolling,
    pollingFromAt,
    statusError,
    loadStatus,
    triggerPush,
    forcePush,
    stopPolling,
    config,
    configLoading,
    configError,
    encryptionKeyAvailable,
    minIntervalMinutes,
    maxIntervalMinutes,
    loadConfig,
    testConfig,
    saveConfig,
    disable,
  } = useBackupStore()

  const [isForcePushing, setIsForcePushing] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [hasSshKey, setHasSshKey] = useState(false)
  const [hasHttpPassword, setHasHttpPassword] = useState(false)
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  const [disabling, setDisabling] = useState(false)
  const [testResult, setTestResult] = useState<
    { ok: true } | { ok: false; message: string } | null
  >(null)

  useSetTitle({ title: t('pageTitle') })

  useEffect(() => {
    loadStatus()
    loadConfig()
  }, [loadStatus, loadConfig])

  // Seed the form from the loaded config once it arrives.
  useEffect(() => {
    if (!config) return
    setForm({
      remoteUrl: config.remoteUrl || '',
      authMode: config.authMode || (config.remoteUrl ? 'https' : 'ssh'),
      branch: config.branch || 'main',
      authorName: config.authorName || '',
      authorEmail: config.authorEmail || '',
      sshKey: '',
      sshKeyPath: config.sshKeyPath || '',
      sshKnownHostsPath: config.sshKnownHostsPath || '',
      httpUsername: config.httpUsername || '',
      httpPassword: '',
      intervalMinutes: String(config.intervalMinutes || 60),
    })
    setHasSshKey(config.hasSshKey)
    setHasHttpPassword(config.hasHttpPassword)
  }, [config])

  useEffect(() => {
    if (!isPolling) return
    const interval = setInterval(() => loadStatus(), POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [isPolling, loadStatus])

  useEffect(() => {
    if (!isPolling) return
    const hasNewBackup = lastBackupAt !== null && pollingFromAt !== lastBackupAt
    const hasError = lastError !== ''
    if (hasNewBackup || hasError) {
      stopPolling()
      if (hasError) {
        toast.error(t('toast.backupFailed', { message: lastError }))
      } else {
        toast.success(t('toast.backupCompleted'))
      }
    }
  }, [lastBackupAt, lastError, isPolling, pollingFromAt, stopPolling, t])

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

  const handlePush = async () => {
    try {
      await triggerPush()
      toast.success(t('toast.backupTriggered'))
    } catch {
      toast.error(t('toast.backupTriggerFailed'))
    }
  }

  const handleForcePush = async () => {
    setIsForcePushing(true)
    try {
      await forcePush()
      toast.success(t('toast.forcePushSuccess'))
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('toast.forcePushFailed'),
      )
    } finally {
      setIsForcePushing(false)
    }
  }

  const showSsh = form.authMode === 'ssh'

  return (
    <div className="settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>
      <p className="settings__section-description">{t('pageDescription')}</p>

      {statusError && (
        <div className="settings__section">
          <p className="text-error text-sm">{statusError}</p>
        </div>
      )}

      {/* Status */}
      <div className="settings__section">
        <h2 className="settings__section-title">{t('sectionTitle')}</h2>

        <div className="settings__preview">
          <span className="settings__preview-label">{t('statusLabel')}</span>
          {isLoading ? (
            <Loader2 className="text-muted h-4 w-4 animate-spin" />
          ) : enabled ? (
            <span className="settings__pill settings__pill-success text-success font-medium">
              {t('statusEnabled')}
            </span>
          ) : (
            <span className="settings__role-pill settings__role-pill--default">
              {t('statusDisabled')}
            </span>
          )}
        </div>

        {(isLoading || enabled) && (
          <div className="settings__preview">
            <span className="settings__preview-label">
              {t('lastBackupLabel')}
            </span>
            <span className="text-interface-text text-sm">
              {isLoading || isPolling ? (
                <span className="text-muted flex items-center gap-2">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  {isPolling ? t('waitingForBackup') : t('loading')}
                </span>
              ) : (
                formatDateTime(lastBackupAt ?? undefined) || t('never')
              )}
            </span>
          </div>
        )}

        {!isLoading && bootError && (
          <div className="settings__preview border-error/20 bg-error/5">
            <span className="settings__preview-label flex items-center gap-1.5">
              <TriangleAlert className="text-error h-3.5 w-3.5" />
              {t('config.bootErrorLabel')}
            </span>
            <span className="text-error text-sm">{bootError}</span>
          </div>
        )}

        {!isLoading && lastError && !needsIntervention && (
          <div className="settings__preview border-error/20 bg-error/5">
            <span className="settings__preview-label flex items-center gap-1.5">
              <TriangleAlert className="text-error h-3.5 w-3.5" />
              {t('lastErrorLabel')}
            </span>
            <span className="text-error text-sm">{lastError}</span>
          </div>
        )}

        {!isLoading && needsIntervention && (
          <div className="settings__preview border-warning/20 bg-warning/5">
            <span className="settings__preview-label flex items-center gap-1.5">
              <GitMerge className="text-warning h-3.5 w-3.5" />
              {t('conflictTitle')}
            </span>
            <div className="flex flex-col gap-2">
              <span className="text-warning text-sm font-medium">
                {t('conflictDescription')}
              </span>
              <span className="text-muted text-xs">{conflictDetails}</span>
              <span className="text-muted text-xs">{t('conflictWarning')}</span>
              <div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleForcePush}
                  disabled={isForcePushing}
                  className="border-warning/40 text-warning hover:bg-warning/10 mt-1"
                >
                  {isForcePushing ? (
                    <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <GitMerge className="mr-2 h-3.5 w-3.5" />
                  )}
                  {isForcePushing ? t('pushing') : t('forcePushButton')}
                </Button>
              </div>
            </div>
          </div>
        )}

        {envManaged && (
          <p className="settings__hint">{t('config.envManagedHint')}</p>
        )}
      </div>

      {/* Manual sync (when running) */}
      {!isLoading && enabled && (
        <div className="settings__section">
          <h2 className="settings__section-title">{t('manualSectionTitle')}</h2>
          <p className="settings__section-description">
            {t('manualSectionDescription')}
          </p>
          <div className="settings__actions">
            <Button
              onClick={handlePush}
              disabled={isPolling || needsIntervention}
            >
              {isPolling ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <CloudUpload className="mr-2 h-4 w-4" />
              )}
              {isPolling ? t('pushing') : t('pushNow')}
            </Button>
          </div>
        </div>
      )}

      {/* Configuration form (settings-managed only) */}
      {!envManaged && (
        <div className="settings__section">
          <h2 className="settings__section-title">
            {t('config.sectionTitle')}
          </h2>
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
            <p className="settings__hint">{t('config.noEncryptionKeyHint')}</p>
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

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="backup-auth-mode">{t('config.authMode')}</Label>
                <Select
                  value={form.authMode || 'ssh'}
                  onValueChange={(v) => set('authMode', v as BackupAuthMode)}
                >
                  <SelectTrigger id="backup-auth-mode">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ssh">
                      {t('config.authModeSsh')}
                    </SelectItem>
                    <SelectItem value="https">
                      {t('config.authModeHttps')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {showSsh ? (
                <>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="backup-ssh-key">{t('config.sshKey')}</Label>
                    <Textarea
                      id="backup-ssh-key"
                      className="font-mono text-xs"
                      rows={4}
                      disabled={!encryptionKeyAvailable}
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
                      placeholder="/etc/leafwiki/id_ed25519"
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
                      placeholder="/etc/leafwiki/known_hosts"
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
                      disabled={!encryptionKeyAvailable}
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
                  placeholder="main"
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
                    placeholder="LeafWiki Backup"
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
                    placeholder="backup@leafwiki.local"
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
                    {disabling && (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    )}
                    {t('config.disableButton')}
                  </Button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
