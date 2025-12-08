// src/features/outline/OutlinePane.tsx
import clsx from 'clsx'
import { Link } from 'react-router'
import { useHeadlinesStore } from '../preview/headlines'

export function OutlinePane() {
  const headlines = useHeadlinesStore((s) => s.headlines)

  if (!headlines.length) {
    return (
      <div className="outline_pane">
        <p className="outline_pane__title">Outline</p>
        <p className="outline_pane__empty">No headings on this page.</p>
      </div>
    )
  }

  return (
    <div className="outline_pane" aria-label="Page outline">
      <p className="outline_pane__title">Outline</p>
      <ul className="outline_pane__list">
        {headlines.map((h) => (
          <li
            key={h.id}
            className={clsx(
              'outline_pane__item',
              h.level === 1 && 'outline_pane__item--level-1',
              h.level === 2 && 'outline_pane__item--level-2',
              h.level >= 3 && 'outline_pane__item--level-3',
            )}
          >
            <Link to={`#${h.slug}`}
              className="outline_pane__link"
            >
              {h.text}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}