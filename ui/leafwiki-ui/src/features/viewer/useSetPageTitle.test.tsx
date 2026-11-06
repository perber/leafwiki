import { Page } from '@/lib/api/pages'
import { useBrandingStore } from '@/stores/branding'
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { useSetPageTitle } from './useSetPageTitle'

const page = { title: 'Setup' } as Page

beforeEach(() => {
  document.title = 'Setup - Team Wiki'
  useBrandingStore.setState({ siteName: 'Team Wiki' })
})

describe('useSetPageTitle', () => {
  it('sets the title of a loaded page', () => {
    document.title = 'Team Wiki'
    renderHook(() => useSetPageTitle({ page, settled: false }))
    expect(document.title).toBe('Setup - Team Wiki')
  })

  it('keeps the title from the server shell while the page loads', () => {
    renderHook(() => useSetPageTitle({ page: null, settled: false }))
    expect(document.title).toBe('Setup - Team Wiki')
  })

  it('falls back to the site name once there is no page to show', () => {
    renderHook(() => useSetPageTitle({ page: null, settled: true }))
    expect(document.title).toBe('Team Wiki')
  })

  it('falls back to the site name by default', () => {
    renderHook(() => useSetPageTitle({ page: null }))
    expect(document.title).toBe('Team Wiki')
  })
})
