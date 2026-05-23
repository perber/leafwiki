import { SIDEBAR_SEARCH_PANEL_ID } from '@/lib/registries'
import { useSidebarStore } from '@/stores/sidebar'
import { Page } from '@/lib/api/pages'
import { ChevronDown, ChevronRight, Tag } from 'lucide-react'
import { useId, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

type Props = {
  page: Page
}

function getEditableProperties(
  properties: Record<string, unknown>,
): [string, string][] {
  return Object.entries(properties)
    .filter(([key]) => !key.toLowerCase().startsWith('leafwiki_'))
    .map(([key, value]) => [key, String(value ?? '')])
}

export function PageMetadata({ page }: Props) {
  const [propsOpen, setPropsOpen] = useState(false)
  const propertiesListId = useId()
  const setSidebarMode = useSidebarStore((s) => s.setSidebarMode)
  const [, setSearchParams] = useSearchParams()

  const tags = page.tags ?? []
  const allProperties = (page.properties ?? {}) as Record<string, unknown>
  const editableProps = getEditableProperties(allProperties)

  const hasTags = tags.length > 0
  const hasProperties = editableProps.length > 0

  if (!hasTags && !hasProperties) return null

  function handleTagClick(tag: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.delete('q')
        next.delete('tags')
        next.append('tags', tag)
        return next
      },
      { replace: true },
    )
    setSidebarMode(SIDEBAR_SEARCH_PANEL_ID)
  }

  return (
    <div
      className={
        hasTags && hasProperties
          ? 'page-metadata page-metadata--two-col'
          : 'page-metadata'
      }
    >
      {hasTags && (
        <div className="page-metadata__tags">
          <Tag size={13} className="page-metadata__tags-icon" />
          <div className="page-metadata__tags-list">
            {tags.map((tag) => (
              <button
                key={tag}
                type="button"
                className="page-metadata__tag-chip"
                onClick={() => handleTagClick(tag)}
              >
                {tag}
              </button>
            ))}
          </div>
        </div>
      )}

      {hasProperties && (
        <div className="page-metadata__properties">
          <button
            type="button"
            className="page-metadata__props-toggle"
            aria-expanded={propsOpen}
            aria-controls={propertiesListId}
            onClick={() => setPropsOpen((o) => !o)}
          >
            {propsOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            <span>Properties ({editableProps.length})</span>
          </button>

          {propsOpen && (
            <dl id={propertiesListId} className="page-metadata__props-list">
              {editableProps.map(([key, value]) => (
                <div key={key} className="page-metadata__prop-row">
                  <dt className="page-metadata__prop-key">{key}</dt>
                  <dd className="page-metadata__prop-value">{value}</dd>
                </div>
              ))}
            </dl>
          )}
        </div>
      )}
    </div>
  )
}
