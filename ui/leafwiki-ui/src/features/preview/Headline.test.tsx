import { render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import Headline from './Headline'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

describe('Headline anchor icon scoping', () => {
  it('marks only the decorative anchor icon with headline-anchor__icon', () => {
    const { container } = render(
      <Headline
        level={2}
        id="heading-with-code"
        data-leafwiki-generated-id="true"
      >
        {'Heading with '}
        <span className="markdown-inline-code" data-testid="content-span">
          <code>inlineCode</code>
        </span>
      </Headline>,
    )

    const icons = container.querySelectorAll('.headline-anchor__icon')
    // Exactly one decorative icon wrapper, and it holds the paperclip svg.
    expect(icons).toHaveLength(1)
    expect(icons[0].querySelector('svg')).not.toBeNull()

    // Regression: the over-broad `.headline-anchor span` selector used to also
    // hit content spans (e.g. inline code) inside a heading and hide them.
    const contentSpan = container.querySelector<HTMLElement>(
      '[data-testid="content-span"]',
    )
    expect(contentSpan).not.toBeNull()
    expect(contentSpan!.classList.contains('headline-anchor__icon')).toBe(false)
    expect(contentSpan!.textContent).toBe('inlineCode')
  })

  it('also scopes the icon in the nested-link branch', () => {
    const { container } = render(
      <Headline
        level={2}
        id="heading-with-link"
        data-leafwiki-generated-id="true"
      >
        <a href="https://example.com">a link</a>
      </Headline>,
    )

    const icons = container.querySelectorAll('.headline-anchor__icon')
    expect(icons).toHaveLength(1)
    expect(icons[0].querySelector('svg')).not.toBeNull()
  })
})
