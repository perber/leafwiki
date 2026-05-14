import { SIDEBAR_TAGS_PANEL_ID } from '@/lib/registries'
import { useTagsStore } from '@/stores/tags'
import { useSidebarStore } from '@/stores/sidebar'
import { Page } from '@/lib/api/pages'
import { ChevronDown, ChevronRight, Tag } from 'lucide-react'
import { useState } from 'react'

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
  const setActiveTags = useTagsStore((s) => s.setActiveTags)
  const setSidebarMode = useSidebarStore((s) => s.setSidebarMode)

  const tags = page.tags ?? []
  const allProperties = (page.properties ?? {}) as Record<string, unknown>
  const editableProps = getEditableProperties(allProperties)

  const hasTags = tags.length > 0
  const hasProperties = editableProps.length > 0

  if (!hasTags && !hasProperties) return null

  function handleTagClick(tag: string) {
    setActiveTags([tag])
    setSidebarMode(SIDEBAR_TAGS_PANEL_ID)
  }

  return (
    <div className="page-metadata">
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
            onClick={() => setPropsOpen((o) => !o)}
          >
            {propsOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            <span>Properties ({editableProps.length})</span>
          </button>

          {propsOpen && (
            <dl className="page-metadata__props-list">
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
