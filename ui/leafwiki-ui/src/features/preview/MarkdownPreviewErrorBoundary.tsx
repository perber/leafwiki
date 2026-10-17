import i18next from '@/lib/i18n'
import { Component, ErrorInfo, ReactNode } from 'react'

type MarkdownPreviewErrorBoundaryState = {
  hasError: boolean
}

export class MarkdownPreviewErrorBoundary extends Component<
  { children: ReactNode; resetKey: string },
  MarkdownPreviewErrorBoundaryState
> {
  state: MarkdownPreviewErrorBoundaryState = { hasError: false }

  static getDerivedStateFromError(): MarkdownPreviewErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Markdown preview failed to render', error, errorInfo)
  }

  componentDidUpdate(prevProps: { children: ReactNode; resetKey: string }) {
    if (this.state.hasError && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ hasError: false })
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="border-destructive/40 bg-destructive/5 text-destructive rounded-md border p-4 text-sm">
          {i18next.t('markdownPreview.renderError', { ns: 'viewer' })}
        </div>
      )
    }

    return this.props.children
  }
}
