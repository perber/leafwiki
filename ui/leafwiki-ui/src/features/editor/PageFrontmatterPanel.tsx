import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { Tag, X } from 'lucide-react'
import { KeyboardEvent, useMemo, useState } from 'react'

type PageFrontmatterPanelProps = {
  tags: string[]
  rawValue: string
  onTagsChange: (tags: string[]) => void
  onRawValueChange: (value: string) => void
}

function normalizeTag(tag: string) {
  return tag.trim()
}

export function PageFrontmatterPanel({
  tags,
  rawValue,
  onTagsChange,
  onRawValueChange,
}: PageFrontmatterPanelProps) {
  const [tagDraft, setTagDraft] = useState('')
  const [advancedOpen, setAdvancedOpen] = useState(false)

  const normalizedTags = useMemo(() => {
    const seen = new Set<string>()
    return tags.filter((tag) => {
      const normalized = normalizeTag(tag)
      if (!normalized) return false
      const key = normalized.toLocaleLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
  }, [tags])

  const commitTag = (value: string) => {
    const normalized = normalizeTag(value)
    if (!normalized) return
    const exists = normalizedTags.some(
      (tag) => tag.toLocaleLowerCase() === normalized.toLocaleLowerCase(),
    )
    if (exists) return
    onTagsChange([...normalizedTags, normalized])
  }

  const handleTagKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' && event.key !== ',') {
      return
    }

    event.preventDefault()
    commitTag(tagDraft)
    setTagDraft('')
  }

  const hasAdvancedValues = rawValue.trim().length > 0

  return (
    <section
      className="page-frontmatter-panel"
      data-testid="page-frontmatter-panel"
    >
      <div className="page-frontmatter-panel__header">
        <div className="page-frontmatter-panel__title-group">
          <span className="page-frontmatter-panel__eyebrow">Page metadata</span>
          <div className="page-frontmatter-panel__title-row">
            <Tag className="page-frontmatter-panel__title-icon" size={16} />
            <span className="page-frontmatter-panel__title">Tags</span>
          </div>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setAdvancedOpen((open) => !open)}
          className={cn(
            'page-frontmatter-panel__toggle',
            advancedOpen && 'page-frontmatter-panel__toggle--open',
          )}
          data-testid="page-frontmatter-toggle"
        >
          {advancedOpen
            ? 'Hide YAML'
            : hasAdvancedValues
              ? 'Edit YAML'
              : 'Add YAML'}
        </Button>
      </div>

      <div className="page-frontmatter-panel__tag-editor">
        <div className="page-frontmatter-panel__chips">
          {normalizedTags.map((tag) => (
            <span key={tag} className="page-frontmatter-panel__chip">
              <span>{tag}</span>
              <button
                type="button"
                className="page-frontmatter-panel__chip-remove"
                onClick={() =>
                  onTagsChange(
                    normalizedTags.filter((current) => current !== tag),
                  )
                }
                aria-label={`Remove tag ${tag}`}
              >
                <X size={12} />
              </button>
            </span>
          ))}
          <Input
            value={tagDraft}
            onChange={(event) => setTagDraft(event.target.value)}
            onKeyDown={handleTagKeyDown}
            onBlur={() => {
              commitTag(tagDraft)
              setTagDraft('')
            }}
            placeholder="Add tag and press Enter"
            className="page-frontmatter-panel__tag-input"
            data-testid="page-frontmatter-tag-input"
          />
        </div>
        <p className="page-frontmatter-panel__hint">
          Tags are always one step away. Use the YAML area for additional
          frontmatter fields when needed.
        </p>
      </div>

      {advancedOpen ? (
        <div className="page-frontmatter-panel__advanced">
          <label
            className="page-frontmatter-panel__advanced-label"
            htmlFor="page-frontmatter-raw-input"
          >
            Additional frontmatter YAML
          </label>
          <textarea
            id="page-frontmatter-raw-input"
            value={rawValue}
            onChange={(event) => onRawValueChange(event.target.value)}
            className="page-frontmatter-panel__textarea"
            placeholder={
              'description: Short summary\nstatus: draft\naliases:\n  - start-here'
            }
            data-testid="page-frontmatter-raw-input"
          />
          <p className="page-frontmatter-panel__hint">
            Keep custom fields here. The editor manages <code>tags</code>{' '}
            separately so they stay easy to edit.
          </p>
        </div>
      ) : null}
    </section>
  )
}
