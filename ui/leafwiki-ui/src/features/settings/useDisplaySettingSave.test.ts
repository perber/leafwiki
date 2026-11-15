import { ApiLocalizedError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useDisplaySettingSave } from './useDisplaySettingSave'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const updatePageDisplay = vi.fn()
const messages = { saved: 'Saved', failed: 'Could not save' }

beforeEach(() => {
  vi.clearAllMocks()
  useConfigStore.setState({ updatePageDisplay })
})

describe('useDisplaySettingSave', () => {
  it('saves the patch and shows the success toast', async () => {
    updatePageDisplay.mockResolvedValueOnce(undefined)
    const { result } = renderHook(() => useDisplaySettingSave(messages))

    await act(() => result.current.save({ showSubpageCount: true }))

    expect(updatePageDisplay).toHaveBeenCalledWith({ showSubpageCount: true })
    expect(toast.success).toHaveBeenCalledWith('Saved')
    expect(toast.error).not.toHaveBeenCalled()
    expect(result.current.saving).toBe(false)
  })

  it.each([
    {
      name: 'a translated server error',
      err: new ApiLocalizedError({
        code: 'page_display_invalid_content_width',
        message: 'Invalid page content width',
        template: 'invalid page content width',
        args: ['huge'],
      }),
      want: expect.stringContaining('Invalid page width'),
    },
    { name: 'a plain error', err: new Error('boom'), want: 'boom' },
    { name: 'a non-error value', err: 'nope', want: 'Could not save' },
  ])('toasts $name through mapApiError', async ({ err, want }) => {
    updatePageDisplay.mockRejectedValueOnce(err)
    const { result } = renderHook(() => useDisplaySettingSave(messages))

    await act(() => result.current.save({ contentWidth: 'huge' as never }))

    expect(toast.error).toHaveBeenCalledWith(want)
    expect(toast.success).not.toHaveBeenCalled()
    expect(result.current.saving).toBe(false)
  })

  it.each([
    { outcome: 'succeeds', settle: (r: Deferred) => r.resolve(undefined) },
    { outcome: 'fails', settle: (r: Deferred) => r.reject(new Error('boom')) },
  ])(
    'is saving while the request runs and resets when it $outcome',
    async ({ settle }) => {
      const pending = deferred()
      updatePageDisplay.mockReturnValueOnce(pending.promise)
      const { result } = renderHook(() => useDisplaySettingSave(messages))

      let saved: Promise<void> = Promise.resolve()
      act(() => {
        saved = result.current.save({ alwaysShowToc: true })
      })
      expect(result.current.saving).toBe(true)

      await act(async () => {
        settle(pending)
        await saved
      })
      expect(result.current.saving).toBe(false)
    },
  )
})

type Deferred = {
  promise: Promise<unknown>
  resolve: (v: unknown) => void
  reject: (e: unknown) => void
}

function deferred(): Deferred {
  let resolve: (v: unknown) => void = () => {}
  let reject: (e: unknown) => void = () => {}
  const promise = new Promise((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}
