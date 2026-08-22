import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { MarkdownImage } from './MarkdownImage'

describe('MarkdownImage', () => {
  beforeEach(() => {
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
  })

  it('opens the image preview dialog when not wrapped in a link', () => {
    render(<MarkdownImage src="/assets/foo.png" alt="foo" />)

    fireEvent.click(screen.getByAltText('foo'))

    expect(useDialogsStore.getState().dialogType).toBe(DIALOG_IMAGE_PREVIEW)
  })

  it('lets the surrounding link handle the click instead of opening the preview', () => {
    render(
      <a href="https://github.com/perber/leafwiki">
        <MarkdownImage src="/assets/foo.png" alt="foo" />
      </a>,
    )

    fireEvent.click(screen.getByAltText('foo'))

    expect(useDialogsStore.getState().dialogType).toBeNull()
  })
})
