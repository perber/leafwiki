import type { PageNode } from '@/lib/api/pages'
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { MarkdownImage } from './MarkdownImage'

function makeSection(path: string): PageNode {
  return {
    id: `section-${path}`,
    title: path,
    slug: path,
    path,
    version: 'v1',
    children: null,
    kind: 'section',
    parentId: null,
  }
}

describe('MarkdownImage', () => {
  beforeEach(() => {
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ byId: {}, byPath: {} })
    window.history.pushState({}, '', '/')
  })

  afterEach(() => {
    useConfigStore.setState({ relativeLinkMode: 'page-folder' })
    useTreeStore.setState({ byId: {}, byPath: {} })
  })

  it('appends a ?v= cache-busting param to an /assets/ src', () => {
    render(<MarkdownImage src="/assets/foo.png" alt="foo" />)

    const src = screen.getByAltText('foo').getAttribute('src') ?? ''
    expect(src).toContain('/assets/foo.png')
    expect(src).toMatch(/\?v=\d+$/)
  })

  it('rewrites a relative src to the /files route in filesystem mode', () => {
    useConfigStore.setState({ relativeLinkMode: 'filesystem' })
    window.history.pushState({}, '', '/e/docs/guide')

    render(<MarkdownImage src="img/run2-06-reset-1-signin.png" alt="shot" />)

    const src = screen.getByAltText('shot').getAttribute('src') ?? ''
    expect(src).toBe('/files/docs/img/run2-06-reset-1-signin.png')
  })

  it('resolves a relative src against the section itself for index pages', () => {
    useConfigStore.setState({ relativeLinkMode: 'filesystem' })
    window.history.pushState({}, '', '/e/docs')
    useTreeStore.setState({
      byId: { 'section-docs': makeSection('docs') },
      byPath: { docs: makeSection('docs') },
    })

    render(<MarkdownImage src="img/shot.png" path="/docs" alt="shot" />)

    const src = screen.getByAltText('shot').getAttribute('src') ?? ''
    expect(src).toBe('/files/docs/img/shot.png')
  })

  it('opens the image preview dialog when not wrapped in a link', () => {
    render(<MarkdownImage src="/assets/foo.png" alt="foo" />)

    fireEvent.click(screen.getByAltText('foo'))

    expect(useDialogsStore.getState().dialogType).toBe(DIALOG_IMAGE_PREVIEW)
  })

  it('renders inline-block so trailing text stays on the same line (#1471)', () => {
    render(<MarkdownImage src="/assets/foo.png" alt="foo" />)

    expect(screen.getByAltText('foo')).toHaveStyle({ display: 'inline-block' })
  })

  it('has no vertical margin so paragraph spacing governs the gap around it (#1524)', () => {
    render(<MarkdownImage src="/assets/foo.png" alt="foo" />)

    expect(screen.getByAltText('foo')).toHaveStyle({
      marginTop: '0px',
      marginBottom: '0px',
    })
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
