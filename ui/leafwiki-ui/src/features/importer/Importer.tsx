import { Button } from '@/components/ui/button'
import { useImportStore } from '@/stores/import'
import { FileUp, Loader2, PlayIcon, UploadIcon, XIcon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

type ResultFilter = 'all' | 'created' | 'skipped' | 'failed'

function getPlanActionLabel(action: 'create' | 'update' | 'skip'): string {
  switch (action) {
    case 'create':
      return 'Create page'
    case 'update':
      return 'Update page'
    case 'skip':
      return 'Skip item'
  }
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
    return 'Needs attention'
  }

  switch (action) {
    case 'created':
      return 'Created'
    case 'updated':
      return 'Updated'
    case 'skipped':
      return 'Skipped'
    case 'conflicted':
      return 'Conflicted'
  }
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
  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: 'Import' })
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
    createImportPlan(zipFile)
  }, [createImportPlan])

  const closeImporter = useCallback(async () => {
    if (importPlan) {
      await cancelImportPlan()
    }
    navigate('/')
  }, [cancelImportPlan, importPlan, navigate])

  const startNewImport = useCallback(async () => {
    if (importPlan) {
      await cancelImportPlan()
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
      ? 'Running'
      : importStatus === 'completed'
        ? 'Completed'
        : importStatus === 'canceled'
          ? 'Canceled'
          : importStatus === 'failed'
            ? 'Failed'
            : importStatus === 'planned'
              ? 'Planned'
              : 'Idle'
  const clearButtonLabel =
    importStatus === 'running'
      ? importPlan?.cancel_requested
        ? 'Cancel Requested'
        : 'Cancel Import'
      : 'Clear Import Plan'
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
      title: 'Select Zip',
      description: 'Choose the package you want to import.',
    },
    {
      number: 2,
      title: 'Review Plan',
      description: 'Check the generated pages and paths.',
    },
    {
      number: 3,
      title: 'Run Import',
      description: 'Watch progress while the importer works.',
    },
    {
      number: 4,
      title: 'Review Result',
      description: 'Inspect imported, skipped, or failed items.',
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
    ? 'Choose a zip file and create an import plan.'
    : importStatus === 'planned'
      ? 'Review the generated items, then start the import.'
      : importStatus === 'running'
        ? importPlan.cancel_requested
          ? 'Cancellation was requested. The importer will stop after the current item finishes.'
          : 'The import is running. You can stay on this page and watch the progress update.'
        : importStatus === 'completed'
          ? 'The import finished successfully. Review the result below, or start a new import with another zip package.'
          : importStatus === 'failed'
            ? 'The import stopped with an error. Check the message and result items below.'
            : importStatus === 'canceled'
              ? 'The import was canceled. Completed work was kept and is shown below.'
              : 'Select a zip file to begin.'

  return (
    <>
      <div className="settings importer">
        <h1 className="settings__title">Import</h1>
        <div className="settings__section">
          <h2 className="settings__section-title">Before You Start</h2>
          <p className="settings__section-description">
            Import a zip archive with Markdown files. LeafWiki first creates a
            review plan, and only imports pages after you explicitly start the
            execution step.
          </p>
          <div className="importer__callout">
            <div className="importer__callout-title">What happens next?</div>
            <div className="importer__callout-body">
              <strong>
                Step {currentStep}
                {currentStepItem ? `: ${currentStepItem.title}` : ''}
              </strong>
              <span>{nextActionText}</span>
            </div>
          </div>
          <div className="importer__info-grid">
            <div className="importer__info-card">
              <div className="importer__info-title">Safe review first</div>
              <div className="importer__info-text">
                Creating the plan does not import pages yet.
              </div>
            </div>
            <div className="importer__info-card">
              <div className="importer__info-title">Links and assets</div>
              <div className="importer__info-text">
                Internal links and embedded assets are rewritten during import.
              </div>
            </div>
            <div className="importer__info-card">
              <div className="importer__info-title">Still in alpha</div>
              <div className="importer__info-text">
                Some Markdown variants may still need manual cleanup afterward.
              </div>
            </div>
          </div>
        </div>
        <div className="settings__section">
          <h2 className="settings__section-title">Import Flow</h2>
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
            <h2 className="settings__section-title">Choose Import Package</h2>
            <p className="settings__section-description">
              Select a zip archive that contains Markdown files. After that, we
              create a preview plan so you can inspect paths, actions, and notes
              before the import runs.
            </p>
            <p className="settings__section-description">
              If you run into problems, unexpected results, or unsupported edge
              cases, please create an issue on our{' '}
              <a
                href="https://github.com/perber/leafwiki/issues"
                target="_blank"
                rel="noopener noreferrer"
              >
                GitHub repository
              </a>
              .
            </p>
            <div className="settings__preview">
              <span className="settings__preview-label">Current Zip:</span>
              {zipFileName ? (
                <>
                  <span className="settings__preview-filename">
                    {zipFileName}
                  </span>
                </>
              ) : (
                <span className="settings__preview-placeholder">
                  No zip file selected
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
                Select Zip File
              </Button>
              <div className="settings__hint">
                Supported input: a single `.zip` archive containing your
                Markdown knowledge base.
              </div>
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
              Import from Zip
            </Button>
            <div className="settings__hint">
              This only creates the review plan. No pages are imported until you
              click `Execute Import Plan`.
            </div>
          </div>
        )}
        {importPlan && showPlanStep && (
          <div className="settings__section">
            <h2 className="settings__section-title">Import Plan</h2>
            <p className="settings__section-description">
              Review what LeafWiki is about to create, update, or skip before
              you start the import.
            </p>
            <div className="importer__status-banner">
              <div className="importer__status-header">
                <div>
                  <div className="settings__preview-label">Current Status</div>
                  <div className="importer__status-title">{statusLabel}</div>
                </div>
                <span className={statusPillClass}>{statusLabel}</span>
              </div>
              <div className="importer__status-meta">
                <span>Plan ID: {importPlan.id}</span>
                {progressLabel ? <span>Progress: {progressLabel}</span> : null}
                {importPlan.total_items > 0 ? (
                  <span>{progressPercent}% complete</span>
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
                <div className="importer__summary-label">Will Create</div>
                <div className="importer__summary-value">
                  {plannedCreateCount}
                </div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">Will Update</div>
                <div className="importer__summary-value">
                  {plannedUpdateCount}
                </div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">Will Skip</div>
                <div className="importer__summary-value">
                  {plannedSkipCount}
                </div>
              </div>
            </div>
            {importStatus === 'running' && (
              <p className="settings__section-description importer__status-copy">
                {importPlan.cancel_requested
                  ? 'Cancellation has been requested. The importer will stop after the current item finishes.'
                  : 'The import is running in the background. You can stay on this page to watch progress.'}
                {importPlan.current_item_source_path
                  ? ` Currently processing ${importPlan.current_item_source_path}.`
                  : ''}
              </p>
            )}
            {importStatus === 'canceled' && (
              <p className="settings__section-description importer__status-copy">
                The import was canceled. Completed items were kept, and the
                partial result is shown below.
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
                        Tree Hash:
                      </th>
                      <td>{importPlan.tree_hash}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">Progress:</th>
                      <td>{progressLabel ?? '0/0'}</td>
                    </tr>
                    {importPlan.started_at && (
                      <tr>
                        <th className="settings__table-header-cell">
                          Started At:
                        </th>
                        <td>
                          {new Date(importPlan.started_at).toLocaleString()}
                        </td>
                      </tr>
                    )}
                    {importPlan.finished_at && (
                      <tr>
                        <th className="settings__table-header-cell">
                          Finished At:
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
                Start New Import
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
                Close and Clear
              </Button>
            </div>
          </div>
        )}
        {importPlan && showPlanStep && importPlan.items.length > 0 && (
          <>
            <div className="settings__section">
              <h2 className="settings__section-title">
                Planned Items ({importPlan.items.length})
              </h2>
              <p className="settings__section-description">
                These are the individual files LeafWiki detected in the package
                and how they map to target pages.
              </p>
              <div className="settings__field">
                <div className="settings__table-card">
                  <div className="settings__table-scroll">
                    <table className="settings__table">
                      <thead className="settings__table-head">
                        <tr>
                          <th className="settings__table-header-cell">
                            Import File
                          </th>
                          <th className="settings__table-header-cell">
                            Target Page
                          </th>
                          <th className="settings__table-header-cell">Title</th>
                          <th className="settings__table-header-cell">Type</th>
                          <th className="settings__table-header-cell">
                            Planned Action
                          </th>
                          <th className="settings__table-header-cell">
                            Notes for Review
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
                                  No special notes
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
            <h2 className="settings__section-title">Run Import</h2>
            <p className="settings__section-description">
              {showPlanStep
                ? 'Start the import once the plan looks correct. You can also clear the current plan and begin again with a different package.'
                : 'The importer is currently working through the generated plan. You can stay on this page to monitor progress or request cancellation.'}
            </p>
            {showRunStep && (
              <>
                <div className="importer__status-banner">
                  <div className="importer__status-header">
                    <div>
                      <div className="settings__preview-label">
                        Current Status
                      </div>
                      <div className="importer__status-title">
                        {statusLabel}
                      </div>
                    </div>
                    <span className={statusPillClass}>{statusLabel}</span>
                  </div>
                  <div className="importer__status-meta">
                    <span>Plan ID: {importPlan.id}</span>
                    {progressLabel ? (
                      <span>Progress: {progressLabel}</span>
                    ) : null}
                    {importPlan.total_items > 0 ? (
                      <span>{progressPercent}% complete</span>
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
                    ? 'Cancellation has been requested. The importer will stop after the current item finishes.'
                    : 'The import is running in the background. You can stay on this page to watch progress.'}
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
                  ? 'Import Running'
                  : 'Execute Import Plan'}
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
            <h2 className="settings__section-title">Import Result</h2>
            <p className="settings__section-description">
              This is the final outcome of the import. Skipped items usually
              need manual review, while failed items need attention before
              retrying.
            </p>
            <div className="importer__callout">
              <div className="importer__callout-title">
                Want to import another package?
              </div>
              <div className="importer__callout-body">
                <span>
                  Start a new import to clear this result and choose a different
                  zip file.
                </span>
              </div>
            </div>
            <div className="importer__summary-grid">
              <div className="importer__summary-card">
                <div className="importer__summary-label">Created</div>
                <div className="importer__summary-value">{createdCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">Skipped</div>
                <div className="importer__summary-value">{skippedCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">Failed</div>
                <div className="importer__summary-value">{failedCount}</div>
              </div>
              <div className="importer__summary-card">
                <div className="importer__summary-label">Tree Updated</div>
                <div className="importer__summary-value">
                  {importResult.tree_hash === importResult.tree_hash_before
                    ? 'No'
                    : 'Yes'}
                </div>
              </div>
            </div>

            <div className="settings__table-card">
              <div className="settings__table-scroll">
                <table className="settings__table">
                  <tbody>
                    <tr>
                      <th className="settings__table-header-cell">
                        Imported Count:
                      </th>
                      <td>{importResult.imported_count}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        Updated Count:
                      </th>
                      <td>{importResult.updated_count}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        Skipped Count:
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
                Start New Import
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
                Close and Clear
              </Button>
            </div>
          </div>
        )}
        {importResult && showResultStep && importResult.items.length > 0 && (
          <div className="settings__section">
            <h2 className="settings__section-title">
              Result Items ({filteredResultItems.length}/
              {importResult.items.length})
            </h2>
            <p className="settings__section-description">
              Use the filters to focus on successful imports, skipped items, or
              failures.
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
                    {filter}
                  </button>
                ))}
              </div>
              <input
                type="search"
                className="importer__search"
                placeholder="Filter by path, action, or error"
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
                        Imported File
                      </th>
                      <th className="settings__table-header-cell">
                        Resulting Page
                      </th>
                      <th className="settings__table-header-cell">Outcome</th>
                      <th className="settings__table-header-cell">Details</th>
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
                            'Imported successfully'
                          ) : item.action === 'updated' ? (
                            'Existing page was updated'
                          ) : item.action === 'skipped' ? (
                            'Skipped without error'
                          ) : (
                            <span className="importer__muted-copy">
                              No extra details
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
                          No items match the current filter.
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
