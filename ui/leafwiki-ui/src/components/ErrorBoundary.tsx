import i18next from '@/lib/i18n'
import { Button } from '@/components/ui/button'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { Component, ErrorInfo } from 'react'
import type { ReactNode } from 'react'

type Props = {
  children: ReactNode
  resetKey?: string
}

type State = {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: unknown): State {
    return { error: error instanceof Error ? error : new Error(String(error)) }
  }

  componentDidUpdate(prevProps: Props) {
    if (this.state.error && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ error: null })
    }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack)
  }

  private handleReload = () => {
    window.location.reload()
  }

  private handleReset = () => {
    this.setState({ error: null })
  }

  render() {
    const { error } = this.state

    if (!error) return this.props.children

    const t = (key: string) =>
      i18next.t(key, { ns: 'common', keyPrefix: 'errorBoundary' })

    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-6 p-8">
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="bg-destructive/10 text-destructive flex h-16 w-16 items-center justify-center rounded-full">
            <AlertTriangle className="h-8 w-8" />
          </div>
          <div className="flex flex-col gap-1">
            <h1 className="text-foreground text-xl font-semibold">
              {t('title')}
            </h1>
            <p className="text-muted-foreground max-w-md text-sm">
              {t('description')}
            </p>
          </div>
        </div>

        <div className="flex gap-3">
          <Button onClick={this.handleReload} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            {t('reload')}
          </Button>
          <Button variant="outline" onClick={this.handleReset}>
            {t('recover')}
          </Button>
        </div>

        <details className="border-border bg-muted/40 w-full max-w-xl rounded-md border">
          <summary className="text-muted-foreground cursor-pointer p-3 text-xs font-medium select-none">
            {t('details')}
          </summary>
          <pre className="text-destructive overflow-auto p-3 pt-0 text-xs break-all whitespace-pre-wrap">
            {error.stack ?? error.message}
          </pre>
        </details>
      </div>
    )
  }
}
