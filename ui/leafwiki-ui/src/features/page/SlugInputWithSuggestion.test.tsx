import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

vi.mock('@/lib/api/pages', () => ({
  suggestSlug: vi.fn(),
}))

vi.mock('@/lib/i18n', () => ({
  default: {
    t: (key: string) => key,
  },
}))

import { suggestSlug } from '@/lib/api/pages'
const mockSuggestSlug = suggestSlug as ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
  mockSuggestSlug.mockResolvedValue('suggested-slug')
})

describe('SlugInputWithSuggestion', () => {
  const defaultProps = {
    title: 'Grafana',
    slug: 'grafana-1',
    parentId: 'root',
    currentId: 'page-123',
    onSlugChange: vi.fn(),
    onLastSlugTitleChange: vi.fn(),
    onSlugLoadingChange: vi.fn(),
    onSlugTouchedChange: vi.fn(),
  }

  describe('initialTitle prop', () => {
    it('does not call suggestSlug when title matches initialTitle', async () => {
      render(
        <SlugInputWithSuggestion {...defaultProps} initialTitle="Grafana" />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(mockSuggestSlug).not.toHaveBeenCalled()
    })

    it('preserves existing slug when title matches initialTitle', async () => {
      const onSlugChange = vi.fn()
      render(
        <SlugInputWithSuggestion
          {...defaultProps}
          slug="grafana-1"
          initialTitle="Grafana"
          onSlugChange={onSlugChange}
        />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(onSlugChange).not.toHaveBeenCalled()
      expect(screen.getByDisplayValue('grafana-1')).toBeTruthy()
    })

    it('still notifies parent that slug title is settled when title matches initialTitle', async () => {
      const onLastSlugTitleChange = vi.fn()
      render(
        <SlugInputWithSuggestion
          {...defaultProps}
          initialTitle="Grafana"
          onLastSlugTitleChange={onLastSlugTitleChange}
        />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(onLastSlugTitleChange).toHaveBeenCalledWith('Grafana')
    })

    it('calls suggestSlug when no initialTitle is provided', async () => {
      render(
        <SlugInputWithSuggestion {...defaultProps} initialTitle={undefined} />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(mockSuggestSlug).toHaveBeenCalledWith(
        'root',
        'Grafana',
        'page-123',
      )
    })

    it('calls suggestSlug when title differs from initialTitle', async () => {
      render(
        <SlugInputWithSuggestion
          {...defaultProps}
          title="Grafana New"
          initialTitle="Grafana"
        />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(mockSuggestSlug).toHaveBeenCalledWith(
        'root',
        'Grafana New',
        'page-123',
      )
    })
  })

  describe('create mode (no initialTitle)', () => {
    it('calls suggestSlug when title is non-empty', async () => {
      render(
        <SlugInputWithSuggestion
          {...defaultProps}
          title="New Page"
          initialTitle={undefined}
        />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(mockSuggestSlug).toHaveBeenCalledWith(
        'root',
        'New Page',
        'page-123',
      )
    })

    it('does not call suggestSlug when title is empty', async () => {
      render(
        <SlugInputWithSuggestion
          {...defaultProps}
          title=""
          initialTitle={undefined}
        />,
      )

      await act(async () => {
        await new Promise((r) => setTimeout(r, 400))
      })

      expect(mockSuggestSlug).not.toHaveBeenCalled()
    })
  })
})
