import { Button } from '@/components/ui/button'
import i18next from '@/lib/i18n'
import { useImportStore } from '@/stores/import'
import { FileUp, Loader2, PlayIcon, UploadIcon, XIcon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

type ResultFilter = 'all' | 'created' | 'skipped' | 'failed'

function getPlanActionLabel(action: 'create' | 'update' | 'skip'): string {
  return i18next.t(`planAction.${action}`, { ns: 'importer' })
}

function getPlanActionClass(action: 'create' | 'update' | 'skip'): string {
  switch (action) {
    case 'create':
      return 'settings__pill settings__pill-success'
    case 'update':
      return 'settings__pill settings__pill-warning'
    case 'skip':
      return 'settings__pill settings__pill-warning'
  }
}

function getResultActionLabel(
  action: 'created' | 'updated' | 'skipped' | 'conflicted',
  hasError: boolean,
): string {
  if (hasError) {
    return i18next.t('resultAction.needsAttention', { ns: 'importer' })
  }

  return i18next.t(`resultAction.${action}`, { ns: 'importer' })
}

function getResultActionClass(
  action: 'created' | 'updated' | 'skipped' | 'conflicted',
  hasError: boolean,
): string {
  if (hasError || action === 'conflicted') {
    return 'settings__pill settings__pill-error'
  }

  switch (action) {
    case 'created':
      return 'settings__pill settings__pill-success'
    case 'updated':
      return 'settings__pill settings__pill-warning'
    case 'skipped':
      return 'settings__pill settings__pill-warning'
  }
}

export default function Importer() {
  const { t } = useTranslation('importer')
  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: t('title') })
  const navigate = useNavigate()
  const zipRef = useRef<HTMLInputElement>(null)
  const [zipFileName, setZipFileName] = useState('')
  const [resultFilter, setResultFilter] = useState<ResultFilter>('all')
  const [resultSearch, setResultSearch] = useState('')

  const createImportPlan = useImportStore((store) => store.createImportPlan)
  const executeImportPlan = useImportStore((store) => store.executeImportPlan)
  const cancelImportPlan = useImportStore((store) => store.cancelImportPlan)
  const importResult = useImportStore((store) => store.importResult)
  const creatingImportPlan = useImportStore((store) => store.creatingImportPlan)
  const loadingImportPlan = useImportStore((store) => store.loadingImportPlan)
  const executingImportPlan = useImportStore(
    (store) => store.executingImportPlan,
  )
  const cancelingImportPlan = useImportStore(
    (store) => store.cancelingImportPlan,
  )

  const importPlan = useImportStore((store) => store.importPlan)
  const loadImportPlan = useImportStore((store) => store.loadImportPlan)

  useEffect(() => {
    void loadImportPlan()
  }, [loadImportPlan])

  const createImportPlanFromZip = useCallback(() => {
    const zipFile = zipRef.current?.files?.[0]
    if (!zipFile) {
      return
    }
    void createImportPlan(zipFile)
  }, [createImportPlan])

  const closeImporter = useCallback(async () => {
    const cleared = importPlan ? await cancelImportPlan() : true
    if (!cleared) {
      return
    }
    navigate('/')
  }, [cancelImportPlan, importPlan, navigate])

  const startNewImport = useCallback(async () => {
    const cleared = importPlan ? await cancelImportPlan() : true
    if (!cleared) {
      return
    }
    if (zipRef.current) {
      zipRef.current.value = ''
    }
    setZipFileName('')
    setResultFilter('all')
    setResultSearch('')
  }, [cancelImportPlan, importPlan])

  const importStatus = importPlan?.execution_status ?? null
  const progressLabel =
    importPlan && importPlan.total_items > 0
      ? `${importPlan.processed_items}/${importPlan.total_items}`
      : null
  const progressPercent =
    importPlan && importPlan.total_items > 0
      ? Math.round((importPlan.processed_items / importPlan.total_items) * 100)
      : 0
  const statusPillClass =
    importStatus === 'completed'
      ? 'settings__pill settings__pill-success'
      : importStatus === 'failed' || importStatus === 'canceled'
        ? 'settings__pill settings__pill-error'
        : importStatus === 'running'
          ? 'settings__pill settings__pill-warning'
          : 'settings__pill settings__pill-success'
  const statusLabel =
    importStatus === 'running'
      ? t('status.running')
      : importStatus === 'completed'
        ? t('status.completed')
        : importStatus === 'canceled'
          ? t('status.canceled')
          : importStatus === 'failed'
            ? t('status.failed')
            : importStatus === 'planned'
              ? t('status.planned')
              : t('status.idle')
  const clearButtonLabel =
    importStatus === 'running'
      ? importPlan?.cancel_requested
        ? t('actions.cancelRequested')
        : t('actions.cancelImport')
      : t('actions.clearImportPlan')
  const currentStep =
    importStatus === 'running'
      ? 3
      : importResult ||
          importStatus === 'completed' ||
          importStatus === 'failed' ||
          importStatus === 'canceled'
        ? 4
        : importPlan
          ? 2
          : 1
  const stepItems = [
    {
      number: 1,
      title: t('steps.selectZip.title'),
      description: t('steps.selectZip.description'),
    },
    {
      number: 2,
      title: t('steps.reviewPlan.title'),
      description: t('steps.reviewPlan.description'),
    },
    {
      number: 3,
      title: t('steps.runImport.title'),
      description: t('steps.runImport.description'),
    },
    {
      number: 4,
      title: t('steps.reviewResult.title'),
      description: t('steps.reviewResult.description'),
    },
  ]
  const planItems = importPlan?.items ?? []
  const resultItems = importResult?.items ?? []
  const plannedCreateCount = planItems.filter(
    (item) => item.action === 'create',
  ).length
  const plannedUpdateCount = planItems.filter(
    (item) => item.action === 'update',
  ).length
  const plannedSkipCount = planItems.filter(
    (item) => item.action === 'skip',
  ).length
  const filteredResultItems = resultItems.filter((item) => {
    const matchesFilter =
      resultFilter === 'all'
        ? true
        : resultFilter === 'failed'
          ? Boolean(item.error)
          : item.action === resultFilter

    if (!matchesFilter) {
      return false
    }

    const normalizedSearch = resultSearch.trim().toLowerCase()
    if (normalizedSearch === '') {
      return true
    }

    return (
      item.source_path.toLowerCase().includes(normalizedSearch) ||
      item.target_path.toLowerCase().includes(normalizedSearch) ||
      item.action.toLowerCase().includes(normalizedSearch) ||
      (item.error ?? '').toLowerCase().includes(normalizedSearch)
    )
  })
  const failedCount = resultItems.filter((item) => Boolean(item.error)).length
  const createdCount = resultItems.filter(
    (item) => item.action === 'created',
  ).length
  const skippedCount = resultItems.filter(
    (item) => item.action === 'skipped',
  ).length
  const currentStepItem = stepItems.find((step) => step.number === currentStep)
  const showPackageStep = currentStep === 1
  const showPlanStep = currentStep === 2
  const showRunStep = currentStep === 3
  const showResultStep = currentStep === 4
  const showRunSection = currentStep === 2 || currentStep === 3
  const nextActionText = !importPlan
    ? t('nextAction.chooseZip')
    : importStatus === 'planned'
      ? t('nextAction.reviewPlan')
      : importStatus === 'running'
        ? importPlan.cancel_requested
          ? t('nextAction.cancelRequested')
          : t('nextAction.running')
        : importStatus === 'completed'
          ? t('nextAction.completed')
          : importStatus === 'failed'
            ? t('nextAction.failed')
            : importStatus === 'canceled'
              ? t('nextAction.canceled')
              : t('nextAction.selectZip')

  const filterLabels: Record<ResultFilter, string> = {
    all: t('result.filterAll'),
    created: t('result.filterCreated'),
    skipped: t('result.filterSkipped'),
    failed: t('result.filterFailed'),
  }

  return (
    <>
      <div className="settings importer">
        <h1 className="settings__title">{t('title')}</h1>
        <div className="settings__section">
          <h2 className="settings__section-title">{t('beforeStart.title')}</h2>
          <p className="settings__section-description">
            {t('beforeStart.description')}
          </p>
          <div className="importer__callout">
            <div className="importer__callout-title">
              {t('beforeStart.whatHappensNext')}
            </div>
            <div className="importer__callout-body">
              <strong>
                {t('beforeStart.stepLabel', {
                  number: currentStep,
                  title: currentStepItem?.title ?? '',
                })}
              </strong>
              <span>{nextActionText}</span>
            </div>
          </div>
          <div className="importer__info-grid">
            <div className="importer__info-card">
              <div className="importer__info-title">
                {t('beforeStart.safeReviewTitle')}
              </div>
              <div className="importer__info-text">
                {t('beforeStart.safeReviewText')}
              </div>
            </div>
            <div className="importer__info-card">
              <div className="importer__info-title">
                {t('beforeStart.linksAssetsTitle')}
              </div>
              <div className="importer__info-text">
                {t('beforeStart.linksAssetsText')}
              </div>
            </div>
            <div className="importer__info-card">
              <div className="importer__info-title">
                {t('beforeStart.alphaTitle')}
              </div>
              <div className="importer__info-text">
                {t('beforeStart.alphaText')}
              </div>
            </div>
          </div>
        </div>
        <div className="settings__section">
          <h2 className="settings__section-title">{t('flow.title')}</h2>
          <div className="importer__steps">
            {stepItems.map((step) => {
              const state =
                step.number < currentStep
                  ? 'done'
                  : step.number === currentStep
                    ? 'active'
                    : 'upcoming'

              return (
                <div
                  key={step.number}
                  className={`importer__step importer__step--${state}`}
                >
                  <div className="importer__step-number">{step.number}</div>
                  <div>
                    <div className="importer__step-title">{step.title}</div>
                    <div className="importer__step-description">
                      {step.description}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
        {showPackageStep && (
          <div className="settings__section">
            <h2 className="settings__section-title">{t('package.title')}</h2>
            <p className="settings__section-description">
              {t('package.description')}
            </p>
            <p className="settings__section-description">
              {t('package.issuesHint')}{' '}
              <a
                href="https://github.com/perber/leafwiki/issues"
                target="_blank"
                rel="noopener noreferrer"
              >
                {t('package.githubRepository')}
              </a>
              .
            </p>
            <div className="settings__preview">
              <span className="settings__preview-label">
                {t('package.currentZip')}
              </span>
              {zipFileName ? (
                <>
                  <span className="settings__preview-filename">
                    {zipFileName}
                  </span>
                </>
              ) : (
                <span className="settings__preview-placeholder">
                  {t('package.noZipSelected')}
                </span>
              )}
            </div>
            <div className="settings__field">
              <input
                type="file"
                ref={zipRef}
                accept=".zip"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  setZipFileName(file?.name ?? '')
                }}
              />
              <Button
                variant="outline"
                onClick={() => {
                  zipRef.current?.click()
                }}
              >
                <FileUp className="mr-2 h-4 w-4" />
                {t('actions.selectZipFile')}
              </Button>
              <div className="settings__hint">{t('package.supportedInput')}</div>
            </div>
            <Button
              variant="default"
              className="settings__save-button"
              onClick={createImportPlanFromZip}
              disabled={
                !zipFileName || creatingImportPlan || executingImportPlan
              }
            >
              {creatingImportPlan || loadingImportPlan ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <UploadIcon className="mr-2 h-4 w-4" />
              )}
              {t('actions.importFromZip')}
            </Button>
            <div className="settings__hint">{t('package.planOnlyHint')}</div>
          </div>
        )}
        {importPlan && showPlanStep && (
          <div className="settings__section">
            <h2 className="settings__section-title">{t('plan.title')}</h2>
            <p className="settings__section-description">
              {t('plan.description')}
            </p>
            <div className="importer__status-banner">
              <div className="importer__status-header">
                <div>
                  <div className="settings__preview-label">
                    {t('plan.currentStatus')}
                  </div>
                  <div className="importer__status-title">{statusLabel}</div>
                </div>
                <span className={statusPillClass}>{statusLabel}</span>
              </div>
              <div className="importer__status-meta">
                <span>{t('plan.planId', { id: importPlan.id })}</span>
                {progressLabel ? (
                  <span>{t('plan.progress', { label: progressLabel })}</span>
                ) : null}
                {importPlan.total_items > 0 ? (
                  <span>{t('plan.percentComplete', { percent: progressPercent })}</span>
                ) : null}
              </div>
              {importPlan.total_items > 0 ? (
                <div className="importer__progress">
                  <div
                    className="importer__progress-bar"
                    style={{ width: `${progressPercent}%` }}
                  />
                </div>
              ) : null}
            </div>
            <div className="importer__summary-grid importer__summary-grid--three">
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('plan.willCreate')}</div>
                <div className="importer__summary-value">
                  {plannedCreateCount}
                </div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('plan.willUpdate')}</div>
                <div className="importer__summary-value">
                  {plannedUpdateCount}
                </div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('plan.willSkip')}</div>
                <div className="importer__summary-value">
                  {plannedSkipCount}
                </div>
              </div>
            </div>
            {importStatus === 'running' && (
              <p className="settings__section-description importer__status-copy">
                {importPlan.cancel_requested
                  ? t('plan.cancelRequestedCopy')
                  : t('plan.runningCopy')}
                {importPlan.current_item_source_path
                  ? t('plan.currentlyProcessing', {
                      path: importPlan.current_item_source_path,
                    })
                  : ''}
              </p>
            )}
            {importStatus === 'canceled' && (
              <p className="settings__section-description importer__status-copy">
                {t('plan.canceledCopy')}
              </p>
            )}
            {importStatus === 'failed' && importPlan.execution_error && (
              <p className="settings__section-description importer__status-copy text-error">
                {importPlan.execution_error}
              </p>
            )}

            <div className="settings__table-card">
              <div className="settings__table-scroll">
                <table className="settings__table">
                  <tbody>
                    <tr>
                      <th className="settings__table-header-cell">ID:</th>
                      <td>{importPlan.id}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('plan.treeHash')}
                      </th>
                      <td>{importPlan.tree_hash}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('plan.progress', { label: progressLabel ?? '0/0' })}
                      </th>
                      <td>{progressLabel ?? '0/0'}</td>
                    </tr>
                    {importPlan.started_at && (
                      <tr>
                        <th className="settings__table-header-cell">
                          {t('plan.startedAt')}
                        </th>
                        <td>
                          {new Date(importPlan.started_at).toLocaleString()}
                        </td>
                      </tr>
                    )}
                    {importPlan.finished_at && (
                      <tr>
                        <th className="settings__table-header-cell">
                          {t('plan.finishedAt')}
                        </th>
                        <td>
                          {new Date(importPlan.finished_at).toLocaleString()}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
            <div className="mt-4 flex gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  void startNewImport()
                }}
                disabled={cancelingImportPlan}
              >
                {cancelingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <UploadIcon className="mr-2 h-4 w-4" />
                )}
                {t('actions.startNewImport')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  void closeImporter()
                }}
                disabled={cancelingImportPlan}
              >
                {cancelingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <XIcon className="mr-2 h-4 w-4" />
                )}
                {t('actions.closeAndClear')}
              </Button>
            </div>
          </div>
        )}
        {importPlan && showPlanStep && importPlan.items.length > 0 && (
          <>
            <div className="settings__section">
              <h2 className="settings__section-title">
                {t('plan.plannedItems', { count: importPlan.items.length })}
              </h2>
              <p className="settings__section-description">
                {t('plan.plannedItemsDescription')}
              </p>
              <div className="settings__field">
                <div className="settings__table-card">
                  <div className="settings__table-scroll">
                    <table className="settings__table">
                      <thead className="settings__table-head">
                        <tr>
                          <th className="settings__table-header-cell">
                            {t('plan.importFile')}
                          </th>
                          <th className="settings__table-header-cell">
                            {t('plan.targetPage')}
                          </th>
                          <th className="settings__table-header-cell">
                            {t('plan.titleColumn')}
                          </th>
                          <th className="settings__table-header-cell">
                            {t('plan.typeColumn')}
                          </th>
                          <th className="settings__table-header-cell">
                            {t('plan.plannedAction')}
                          </th>
                          <th className="settings__table-header-cell">
                            {t('plan.notesForReview')}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {importPlan.items.map((item) => (
                          <tr key={`${item.source_path}::${item.target_path}`}>
                            <td className="settings__table-cell">
                              {item.source_path}
                            </td>
                            <td className="settings__table-cell">
                              {item.target_path}
                            </td>
                            <td className="settings__table-cell">
                              {item.title}
                            </td>
                            <td className="settings__table-cell">
                              <span className="settings__pill settings__pill-success">
                                {item.kind}
                              </span>
                            </td>
                            <td className="settings__table-cell">
                              <span className={getPlanActionClass(item.action)}>
                                {getPlanActionLabel(item.action)}
                              </span>
                            </td>
                            <td className="settings__table-cell importer__details-cell">
                              {item.notes && item.notes.length > 0 ? (
                                item.notes.join(', ')
                              ) : (
                                <span className="importer__muted-copy">
                                  {t('plan.noSpecialNotes')}
                                </span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
        {importPlan && showRunSection && (
          <div className="settings__section">
            <h2 className="settings__section-title">{t('run.title')}</h2>
            <p className="settings__section-description">
              {showPlanStep ? t('run.descriptionPlan') : t('run.descriptionRunning')}
            </p>
            {showRunStep && (
              <>
                <div className="importer__status-banner">
                  <div className="importer__status-header">
                    <div>
                      <div className="settings__preview-label">
                        {t('plan.currentStatus')}
                      </div>
                      <div className="importer__status-title">
                        {statusLabel}
                      </div>
                    </div>
                    <span className={statusPillClass}>{statusLabel}</span>
                  </div>
                  <div className="importer__status-meta">
                    <span>{t('plan.planId', { id: importPlan.id })}</span>
                    {progressLabel ? (
                      <span>{t('plan.progress', { label: progressLabel })}</span>
                    ) : null}
                    {importPlan.total_items > 0 ? (
                      <span>
                        {t('plan.percentComplete', { percent: progressPercent })}
                      </span>
                    ) : null}
                  </div>
                  {importPlan.total_items > 0 ? (
                    <div className="importer__progress">
                      <div
                        className="importer__progress-bar"
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                  ) : null}
                </div>
                <p className="settings__section-description importer__status-copy">
                  {importPlan.cancel_requested
                    ? t('plan.cancelRequestedCopy')
                    : t('plan.runningCopy')}
                  {importPlan.current_item_source_path
                    ? ` Currently processing ${importPlan.current_item_source_path}.`
                    : ''}
                </p>
              </>
            )}
            <div className="flex gap-2">
              <Button
                variant="default"
                onClick={() => {
                  executeImportPlan()
                }}
                disabled={
                  importPlan.items.length === 0 ||
                  executingImportPlan ||
                  importStatus === 'completed' ||
                  importStatus === 'canceled' ||
                  creatingImportPlan ||
                  cancelingImportPlan
                }
              >
                {executingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <PlayIcon className="mr-2 h-4 w-4" />
                )}
                {importStatus === 'running'
                  ? t('actions.importRunning')
                  : t('actions.executeImportPlan')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  cancelImportPlan()
                }}
                disabled={
                  cancelingImportPlan ||
                  (importStatus === 'running' && importPlan.cancel_requested) ||
                  creatingImportPlan
                }
              >
                {cancelingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <XIcon className="mr-2 h-4 w-4" />
                )}
                {clearButtonLabel}
              </Button>
            </div>
          </div>
        )}
        {importResult && showResultStep && (
          <div className="settings__section">
            <h2 className="settings__section-title">{t('result.title')}</h2>
            <p className="settings__section-description">
              {t('result.description')}
            </p>
            <div className="importer__callout">
              <div className="importer__callout-title">
                {t('result.anotherPackageTitle')}
              </div>
              <div className="importer__callout-body">
                <span>{t('result.anotherPackageBody')}</span>
              </div>
            </div>
            <div className="importer__summary-grid">
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('result.created')}</div>
                <div className="importer__summary-value">{createdCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('result.skipped')}</div>
                <div className="importer__summary-value">{skippedCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">{t('result.failed')}</div>
                <div className="importer__summary-value">{failedCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">
                  {t('result.treeUpdated')}
                </div>
                <div className="importer__summary-value">
                  {importResult.tree_hash === importResult.tree_hash_before
                    ? t('result.treeUpdatedNo')
                    : t('result.treeUpdatedYes')}
                </div>
              </div>
            </div>

            <div className="settings__table-card">
              <div className="settings__table-scroll">
                <table className="settings__table">
                  <tbody>
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('result.importedCount')}
                      </th>
                      <td>{importResult.imported_count}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('result.updatedCount')}
                      </th>
                      <td>{importResult.updated_count}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('result.skippedCount')}
                      </th>
                      <td>{importResult.skipped_count}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div className="mt-4 flex gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  void startNewImport()
                }}
                disabled={cancelingImportPlan}
              >
                {cancelingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <UploadIcon className="mr-2 h-4 w-4" />
                )}
                {t('actions.startNewImport')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  void closeImporter()
                }}
                disabled={cancelingImportPlan}
              >
                {cancelingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <XIcon className="mr-2 h-4 w-4" />
                )}
                {t('actions.closeAndClear')}
              </Button>
            </div>
          </div>
        )}
        {importResult && showResultStep && importResult.items.length > 0 && (
          <div className="settings__section">
            <h2 className="settings__section-title">
              {t('result.resultItems', {
                filtered: filteredResultItems.length,
                total: importResult.items.length,
              })}
            </h2>
            <p className="settings__section-description">
              {t('result.filterDescription')}
            </p>
            <div className="importer__result-toolbar">
              <div className="importer__filter-group">
                {(
                  ['all', 'created', 'skipped', 'failed'] as ResultFilter[]
                ).map((filter) => (
                  <button
                    key={filter}
                    type="button"
                    className={`importer__filter-chip ${
                      resultFilter === filter
                        ? 'importer__filter-chip--active'
                        : ''
                    }`}
                    onClick={() => {
                      setResultFilter(filter)
                    }}
                  >
                    {filterLabels[filter]}
                  </button>
                ))}
              </div>
              <input
                type="search"
                className="importer__search"
                placeholder={t('result.filterPlaceholder')}
                value={resultSearch}
                onChange={(e) => {
                  setResultSearch(e.target.value)
                }}
              />
            </div>
            <div className="settings__table-card">
              <div className="settings__table-scroll">
                <table className="settings__table">
                  <thead className="settings__table-head">
                    <tr>
                      <th className="settings__table-header-cell">
                        {t('result.importedFile')}
                      </th>
                      <th className="settings__table-header-cell">
                        {t('result.resultingPage')}
                      </th>
                      <th className="settings__table-header-cell">
                        {t('result.outcome')}
                      </th>
                      <th className="settings__table-header-cell">
                        {t('result.details')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredResultItems.map((item) => (
                      <tr key={`${item.source_path}::${item.target_path}`}>
                        <td className="settings__table-cell">
                          {item.source_path}
                        </td>
                        <td className="settings__table-cell">
                          {item.target_path}
                        </td>
                        <td className="settings__table-cell">
                          <span
                            className={getResultActionClass(
                              item.action,
                              Boolean(item.error),
                            )}
                          >
                            {getResultActionLabel(
                              item.action,
                              Boolean(item.error),
                            )}
                          </span>
                        </td>
                        <td className="settings__table-cell importer__details-cell">
                          {item.error ? (
                            item.error
                          ) : item.action === 'created' ? (
                            t('result.importedSuccessfully')
                          ) : item.action === 'updated' ? (
                            t('result.existingPageUpdated')
                          ) : item.action === 'skipped' ? (
                            t('result.skippedWithoutError')
                          ) : (
                            <span className="importer__muted-copy">
                              {t('result.noExtraDetails')}
                            </span>
                          )}
                        </td>
                      </tr>
                    ))}
                    {filteredResultItems.length === 0 && (
                      <tr>
                        <td
                          className="settings__table-body-message"
                          colSpan={4}
                        >
                          {t('result.noMatchingItems')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  )
}
