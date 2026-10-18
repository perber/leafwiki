import { beforeEach, describe, expect, it, vi } from 'vitest'

const fetchBackupStatus = vi.fn()

vi.mock('@/lib/api/backup', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api/backup')>()),
  fetchBackupStatus: () => fetchBackupStatus(),
}))

import { useBackupStore } from './backup'

describe('backup store conflictKind', () => {
  beforeEach(() => {
    fetchBackupStatus.mockReset()
    useBackupStore.setState({ conflictKind: null, needsIntervention: false })
  })

  it.each([
    {
      name: 'diverged',
      status: { conflictKind: 'diverged' },
      want: 'diverged',
    },
    {
      name: 'fileConflict',
      status: { conflictKind: 'fileConflict' },
      want: 'fileConflict',
    },
    // The server omits conflictKind once the conflict is resolved
    // (omitempty); a stale kind must not survive the reload.
    { name: 'omitted after resolution', status: {}, want: null },
    { name: 'no status at all', status: undefined, want: null },
  ])('maps $name to $want', async ({ status, want }) => {
    useBackupStore.setState({ conflictKind: 'diverged' })
    fetchBackupStatus.mockResolvedValue({ enabled: true, status })
    await useBackupStore.getState().loadStatus()
    expect(useBackupStore.getState().conflictKind).toBe(want)
  })
})
