import { type ElementType, type ReactNode } from 'react'

type ListViewProps = {
  header?: ReactNode
  footer?: ReactNode
  children: ReactNode
  as?: ElementType
  className?: string
  contentClassName?: string
  testId?: string
}

type ListViewItemProps = {
  children: ReactNode
  active?: boolean
  className?: string
  onClick?: () => void
  testId?: string
}

type ListViewStatusProps = {
  children: ReactNode
  error?: boolean
  className?: string
}

export function ListView({
  header,
  footer,
  children,
  as: Component = 'aside',
  className = '',
  contentClassName = '',
  testId,
}: ListViewProps) {
  return (
    <Component className={`list-view ${className}`.trim()} data-testid={testId}>
      {header ? <div className="list-view__header">{header}</div> : null}
      <div
        className={`list-view__content custom-scrollbar ${contentClassName}`.trim()}
      >
        {children}
      </div>
      {footer ? <div className="list-view__footer">{footer}</div> : null}
    </Component>
  )
}

export function ListViewHeader({ children }: { children: ReactNode }) {
  return <div className="list-view__title">{children}</div>
}

export function ListViewList({ children }: { children: ReactNode }) {
  return <div className="list-view__list">{children}</div>
}

export function ListViewItem({
  children,
  active = false,
  className = '',
  onClick,
  testId,
}: ListViewItemProps) {
  return (
    <button
      type="button"
      className={`list-view__item ${active ? 'list-view__item--active' : ''} ${className}`.trim()}
      onClick={onClick}
      data-testid={testId}
    >
      {children}
    </button>
  )
}

export function ListViewStatus({
  children,
  error = false,
  className = '',
}: ListViewStatusProps) {
  return (
    <div
      className={`list-view__status ${error ? 'list-view__status--error' : ''} ${className}`.trim()}
    >
      {children}
    </div>
  )
}
