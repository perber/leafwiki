import type { ImportPlan, ImportResult } from '@/lib/api/import'
import { describe, expect, it } from 'vitest'
import {
  filterResultItems,
  getClearButtonLabelKey,
  getImportProgress,
  getImportStep,
  getNextActionKey,
  getStatusLabelKey,
  getStatusPillClass,
  type ResultFilter,
} from './importerState'

function plan(overrides: Partial<ImportPlan> = {}): ImportPlan {
  return {
    id: 'p',
    tree_hash: 'h',
    items: [],
    errors: [],
    execution_status: 'planned',
    cancel_requested: false,
    processed_items: 0,
    total_items: 0,
    ...overrides,
  }
}

const result: ImportResult = {
  imported_count: 1,
  updated_count: 0,
  skipped_count: 1,
  items: [
    { source_path: 'docs/a.md', target_path: '/docs/a', action: 'created' },
    { source_path: 'docs/B.md', target_path: '/docs/b', action: 'skipped' },
    {
      source_path: 'x.md',
      target_path: '/x',
      action: 'conflicted',
      error: 'Slug Taken',
    },
  ],
  tree_hash: 'h2',
  tree_hash_before: 'h',
}

const importerLocales = import.meta.glob<Record<string, unknown>>(
  '/src/locales/*/importer.json',
  { eager: true, import: 'default' },
)

const statuses = [
  'planned',
  'running',
  'completed',
  'failed',
  'canceled',
] as const

function lookup(locale: unknown, key: string): unknown {
  return key
    .split('.')
    .reduce<unknown>(
      (node, part) =>
        node && typeof node === 'object'
          ? (node as Record<string, unknown>)[part]
          : undefined,
      locale,
    )
}

describe('importer view state', () => {
  it('derives the current step from plan status and result', () => {
    expect(getImportStep(null, null)).toBe(1)
    expect(getImportStep(plan(), null)).toBe(2)
    expect(getImportStep(plan({ execution_status: 'running' }), result)).toBe(3)
    for (const status of ['completed', 'failed', 'canceled'] as const) {
      expect(getImportStep(plan({ execution_status: status }), null)).toBe(4)
    }
    expect(getImportStep(plan(), result)).toBe(4)
    expect(getImportStep(null, result)).toBe(4)
  })

  it('maps status to pill class and label', () => {
    expect(statuses.map(getStatusPillClass)).toEqual([
      'settings__pill settings__pill-success',
      'settings__pill settings__pill-warning',
      'settings__pill settings__pill-success',
      'settings__pill settings__pill-error',
      'settings__pill settings__pill-error',
    ])
    expect(getStatusPillClass(null)).toBe(
      'settings__pill settings__pill-success',
    )
    expect([
      ...statuses.map(getStatusLabelKey),
      getStatusLabelKey(null),
    ]).toEqual([
      'status.planned',
      'status.running',
      'status.completed',
      'status.failed',
      'status.canceled',
      'status.idle',
    ])
  })

  it('derives clear-button and next-action keys including cancellation', () => {
    expect(getClearButtonLabelKey(null)).toBe('run.clearImportPlan')
    expect(getClearButtonLabelKey(plan())).toBe('run.clearImportPlan')
    expect(getClearButtonLabelKey(plan({ execution_status: 'running' }))).toBe(
      'run.cancelImport',
    )
    expect(
      getClearButtonLabelKey(
        plan({ execution_status: 'running', cancel_requested: true }),
      ),
    ).toBe('run.cancelRequested')

    expect(getNextActionKey(null)).toBe('nextAction.chooseZip')
    expect(
      statuses.map((status) =>
        getNextActionKey(plan({ execution_status: status })),
      ),
    ).toEqual([
      'nextAction.reviewThenStart',
      'nextAction.running',
      'nextAction.completed',
      'nextAction.failed',
      'nextAction.canceled',
    ])
    expect(
      getNextActionKey(
        plan({ execution_status: 'running', cancel_requested: true }),
      ),
    ).toBe('nextAction.cancellationRequested')
  })

  it('returns only keys that exist in every shipped locale', () => {
    const keys = new Set<string>([
      ...statuses.map(getStatusLabelKey),
      getStatusLabelKey(null),
      getClearButtonLabelKey(null),
      getClearButtonLabelKey(plan({ execution_status: 'running' })),
      getClearButtonLabelKey(
        plan({ execution_status: 'running', cancel_requested: true }),
      ),
      getNextActionKey(null),
      getNextActionKey(
        plan({ execution_status: 'running', cancel_requested: true }),
      ),
      ...statuses.map((status) =>
        getNextActionKey(plan({ execution_status: status })),
      ),
    ])
    const locales = Object.entries(importerLocales)
    expect(locales.length).toBeGreaterThanOrEqual(3)
    for (const [file, locale] of locales) {
      for (const key of keys) {
        expect(typeof lookup(locale, key), `${file}: ${key}`).toBe('string')
      }
    }
  })

  it('reports progress only when the plan has items', () => {
    expect(getImportProgress(null)).toEqual({ label: null, percent: 0 })
    expect(getImportProgress(plan())).toEqual({ label: null, percent: 0 })
    expect(
      getImportProgress(plan({ processed_items: 1, total_items: 3 })),
    ).toEqual({ label: '1/3', percent: 33 })
  })

  it('filters results by kind and case-insensitive trimmed search', () => {
    const paths = (items: ImportResult['items']) =>
      items.map((i) => i.source_path)
    expect(paths(filterResultItems(result.items, 'all', ''))).toEqual([
      'docs/a.md',
      'docs/B.md',
      'x.md',
    ])
    expect(paths(filterResultItems(result.items, 'created', ''))).toEqual([
      'docs/a.md',
    ])
    expect(paths(filterResultItems(result.items, 'failed', ''))).toEqual([
      'x.md',
    ])
    expect(paths(filterResultItems(result.items, 'all', '  docs/b '))).toEqual([
      'docs/B.md',
    ])
    expect(paths(filterResultItems(result.items, 'all', 'slug taken'))).toEqual(
      ['x.md'],
    )
    expect(paths(filterResultItems(result.items, 'skipped', 'a.md'))).toEqual(
      [],
    )
  })
})

describe('importer view state edge cases', () => {
  // Statuses outside the typed union can still arrive from an older/newer server.
  const unknown = plan({
    execution_status: 'queued' as unknown as ImportPlan['execution_status'],
  })

  it('falls back for an unknown execution status', () => {
    expect(getImportStep(unknown, null)).toBe(2)
    expect(getStatusLabelKey(unknown.execution_status)).toBe('status.idle')
    expect(getStatusPillClass(unknown.execution_status)).toBe(
      'settings__pill settings__pill-success',
    )
    expect(getNextActionKey(unknown)).toBe('nextAction.selectZipToBegin')
    expect(getClearButtonLabelKey(unknown)).toBe('run.clearImportPlan')
    for (const [file, locale] of Object.entries(importerLocales)) {
      expect(
        typeof lookup(locale, 'nextAction.selectZipToBegin'),
        `${file}: nextAction.selectZipToBegin`,
      ).toBe('string')
    }
  })

  it('ignores cancel_requested unless the import is running', () => {
    for (const status of [
      'planned',
      'completed',
      'failed',
      'canceled',
    ] as const) {
      const p = plan({ execution_status: status, cancel_requested: true })
      expect(getClearButtonLabelKey(p)).toBe('run.clearImportPlan')
      expect(getNextActionKey(p)).not.toBe('nextAction.cancellationRequested')
    }
    expect(
      getImportStep(
        plan({ execution_status: 'running', cancel_requested: true }),
        null,
      ),
    ).toBe(3)
  })

  it.each([
    [0, 5, '0/5', 0],
    [2, 3, '2/3', 67],
    [1, 200, '1/200', 1],
    [1, 201, '1/201', 0],
    [5, 5, '5/5', 100],
    [5, 4, '5/4', 125],
  ])('progress %i/%i -> %s, %i%%', (processed, total, label, percent) => {
    expect(
      getImportProgress(
        plan({ processed_items: processed, total_items: total }),
      ),
    ).toEqual({ label, percent })
  })

  const items: ImportResult['items'] = [
    {
      source_path: 'Ärger/Über.md',
      target_path: '/aerger/ueber',
      action: 'created',
    },
    {
      source_path: 'win\\dir\\a.b.md',
      target_path: '/win/dir/a-b',
      action: 'updated',
    },
    {
      source_path: 'c.md',
      target_path: '/c',
      action: 'created',
      error: 'write failed',
    },
    { source_path: 'd.md', target_path: '/d', action: 'conflicted' },
  ]
  const paths = (list: ImportResult['items']) => list.map((i) => i.source_path)

  it.each<[ResultFilter, string, string[]]>([
    ['all', '   ', ['Ärger/Über.md', 'win\\dir\\a.b.md', 'c.md', 'd.md']],
    ['all', 'ärger/ÜBER', ['Ärger/Über.md']],
    ['all', 'dir\\A.B', ['win\\dir\\a.b.md']],
    ['all', '.*', []],
    ['all', 'conflicted', ['d.md']],
    ['all', 'x'.repeat(10_000), []],
    ['failed', '', ['c.md']],
    ['created', '', ['Ärger/Über.md', 'c.md']],
    ['skipped', '', []],
  ])('filter %s with search %j', (filter, search, expected) => {
    expect(paths(filterResultItems(items, filter, search))).toEqual(expected)
  })

  it('does not mutate the input and handles an empty list', () => {
    const copy = structuredClone(items)
    filterResultItems(items, 'failed', 'c')
    expect(items).toEqual(copy)
    expect(filterResultItems([], 'all', 'a')).toEqual([])
  })
})
