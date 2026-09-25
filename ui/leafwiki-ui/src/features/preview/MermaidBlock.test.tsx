import { fireEvent, render, screen } from '@testing-library/react'
import { useEffect } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MermaidBlock from './MermaidBlock'
import type { MermaidInjectorOps } from './useMermaidInjector'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/lib/i18n', () => ({
  default: { t: (key: string) => key },
}))

const mermaidInjectorSpy = vi.fn<(ops: MermaidInjectorOps) => void>()

vi.mock('./useMermaidInjector', () => ({
  // Real hooks fire their effects once on mount, not on every render — mirror
  // that here, otherwise a mock that calls onError synchronously during
  // render triggers an infinite render loop.
  useMermaidInjector: (ops: MermaidInjectorOps) => {
    useEffect(() => {
      mermaidInjectorSpy(ops)
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])
  },
}))

describe('MermaidBlock', () => {
  beforeEach(() => {
    mermaidInjectorSpy.mockReset()
  })

  it('shows the plain render-error box for a genuine syntax/render error, with no reload button (regression: pins current behavior)', () => {
    mermaidInjectorSpy.mockImplementation(({ onError }) => {
      onError({ message: 'Parse error on line 1', isChunkLoadError: false })
    })

    render(<MermaidBlock code="graph LR\na---b" theme="default" />)

    expect(screen.getByText('mermaid.renderError')).toBeInTheDocument()
    expect(screen.getByText('Parse error on line 1')).toBeInTheDocument()
    expect(
      screen.queryByText('errorBoundary.reloadButton'),
    ).not.toBeInTheDocument()
  })

  it('shows a reload affordance when the diagram fails due to a stale chunk after a redeploy', () => {
    mermaidInjectorSpy.mockImplementation(({ onError }) => {
      onError({
        message:
          'Failed to fetch dynamically imported module: https://example.com/static/flowDiagram-abc.js',
        isChunkLoadError: true,
      })
    })

    const reloadSpy = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, reload: reloadSpy },
    })

    render(<MermaidBlock code="graph LR\na---b" theme="default" />)

    expect(screen.getByText('mermaid.chunkLoadError')).toBeInTheDocument()
    // The raw browser error text is not shown for this case — the actionable
    // "reload" message replaces it instead.
    expect(screen.queryByText(/Failed to fetch/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('errorBoundary.reloadButton'))
    expect(reloadSpy).toHaveBeenCalledOnce()
  })

  it('renders nothing but the diagram container when there is no error', () => {
    mermaidInjectorSpy.mockImplementation(() => {})

    render(<MermaidBlock code="graph LR\na---b" theme="default" />)

    expect(screen.queryByText('mermaid.renderError')).not.toBeInTheDocument()
    expect(screen.queryByText('mermaid.chunkLoadError')).not.toBeInTheDocument()
  })
})
