import type { ImportPlan, ImportResult } from '@/lib/api/import'

// Pure view-state derivation for the importer page. Labels are returned as
// i18n keys in the "importer" namespace so they can be tested without i18n.

export type ResultFilter = 'all' | 'created' | 'skipped' | 'failed'
export type ImportStep = 1 | 2 | 3 | 4
type ResultItem = ImportResult['items'][number]

export function getImportStep(
  importPlan: ImportPlan | null,
  importResult: ImportResult | null,
): ImportStep {
  const importStatus = importPlan?.execution_status ?? null
  return importStatus === 'running'
    ? 3
    : importResult ||
        importStatus === 'completed' ||
        importStatus === 'failed' ||
        importStatus === 'canceled'
      ? 4
      : importPlan
        ? 2
        : 1
}

export function getImportProgress(importPlan: ImportPlan | null): {
  label: string | null
  percent: number
} {
  return {
    label:
      importPlan && importPlan.total_items > 0
        ? `${importPlan.processed_items}/${importPlan.total_items}`
        : null,
    percent:
      importPlan && importPlan.total_items > 0
        ? Math.round(
            (importPlan.processed_items / importPlan.total_items) * 100,
          )
        : 0,
  }
}

export function getStatusPillClass(
  importStatus: ImportPlan['execution_status'] | null,
): string {
  return importStatus === 'completed'
    ? 'settings__pill settings__pill-success'
    : importStatus === 'failed' || importStatus === 'canceled'
      ? 'settings__pill settings__pill-error'
      : importStatus === 'running'
        ? 'settings__pill settings__pill-warning'
        : 'settings__pill settings__pill-success'
}

export function getStatusLabelKey(
  importStatus: ImportPlan['execution_status'] | null,
): string {
  return importStatus === 'running'
    ? 'status.running'
    : importStatus === 'completed'
      ? 'status.completed'
      : importStatus === 'canceled'
        ? 'status.canceled'
        : importStatus === 'failed'
          ? 'status.failed'
          : importStatus === 'planned'
            ? 'status.planned'
            : 'status.idle'
}

export function getClearButtonLabelKey(importPlan: ImportPlan | null): string {
  const importStatus = importPlan?.execution_status ?? null
  return importStatus === 'running'
    ? importPlan?.cancel_requested
      ? 'run.cancelRequested'
      : 'run.cancelImport'
    : 'run.clearImportPlan'
}

export function getNextActionKey(importPlan: ImportPlan | null): string {
  const importStatus = importPlan?.execution_status ?? null
  return !importPlan
    ? 'nextAction.chooseZip'
    : importStatus === 'planned'
      ? 'nextAction.reviewThenStart'
      : importStatus === 'running'
        ? importPlan.cancel_requested
          ? 'nextAction.cancellationRequested'
          : 'nextAction.running'
        : importStatus === 'completed'
          ? 'nextAction.completed'
          : importStatus === 'failed'
            ? 'nextAction.failed'
            : importStatus === 'canceled'
              ? 'nextAction.canceled'
              : 'nextAction.selectZipToBegin'
}

export function filterResultItems(
  resultItems: ResultItem[],
  resultFilter: ResultFilter,
  resultSearch: string,
): ResultItem[] {
  return resultItems.filter((item) => {
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
}
