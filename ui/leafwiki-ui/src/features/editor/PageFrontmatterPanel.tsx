import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  EditorFrontmatterField,
  EditorFrontmatterFieldType,
} from './frontmatter'
import { Plus, Tag, Trash2, X } from 'lucide-react'
import { KeyboardEvent, useMemo, useState } from 'react'

type PageFrontmatterPanelProps = {
  tags: string[]
  fields: EditorFrontmatterField[]
  hasUnsupportedFields: boolean
  onTagsChange: (tags: string[]) => void
  onFieldsChange: (fields: EditorFrontmatterField[]) => void
}

function normalizeTag(tag: string) {
  return tag.trim()
}

function buildEmptyField(): EditorFrontmatterField {
  return {
    key: '',
    type: 'text',
    value: '',
  }
}

function valuePlaceholder(type: EditorFrontmatterFieldType) {
  switch (type) {
    case 'number':
      return '42'
    case 'boolean':
      return 'true'
    case 'list':
      return 'one item per line'
    default:
      return 'Value'
  }
}

function getFieldValidation(
  field: EditorFrontmatterField,
  fields: EditorFrontmatterField[],
  index: number,
) {
  const key = field.key.trim()
  if (!key) return 'Missing key'

  const duplicate = fields.some(
    (candidate, candidateIndex) =>
      candidateIndex !== index &&
      candidate.key.trim().toLocaleLowerCase() === key.toLocaleLowerCase(),
  )
  if (duplicate) return 'Duplicate key'

  if (field.type === 'number' && field.value.trim() !== '') {
    return Number.isNaN(Number(field.value.trim())) ? 'Invalid number' : 'OK'
  }

  if (field.type === 'list') {
    return field.value
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean).length > 0
      ? 'OK'
      : 'Empty list'
  }

  if (field.type === 'text' && field.value.trim() === '') {
    return 'Empty value'
  }

  return 'OK'
}

export function PageFrontmatterPanel({
  tags,
  fields,
  hasUnsupportedFields,
  onTagsChange,
  onFieldsChange,
}: PageFrontmatterPanelProps) {
  const [tagDraft, setTagDraft] = useState('')

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

  const updateField = (
    index: number,
    patch: Partial<EditorFrontmatterField>,
  ) => {
    const next = fields.map((field, currentIndex) =>
      currentIndex === index ? { ...field, ...patch } : field,
    )
    onFieldsChange(next)
  }

  const removeField = (index: number) => {
    onFieldsChange(fields.filter((_, currentIndex) => currentIndex !== index))
  }

  const addField = () => {
    onFieldsChange([...fields, buildEmptyField()])
  }

  const summaryParts = [
    normalizedTags.length === 1 ? '1 tag' : `${normalizedTags.length} tags`,
    fields.length === 1 ? '1 property' : `${fields.length} properties`,
  ]

  return (
    <section
      className="page-frontmatter-panel"
      data-testid="page-frontmatter-panel"
    >
      <Accordion
        type="single"
        collapsible
        className="page-frontmatter-panel__accordion"
      >
        <AccordionItem
          value="metadata"
          className="page-frontmatter-panel__item"
        >
          <AccordionTrigger className="page-frontmatter-panel__trigger">
            <div className="page-frontmatter-panel__topline">
              <div className="page-frontmatter-panel__title-row">
                <Tag className="page-frontmatter-panel__title-icon" size={14} />
                <span className="page-frontmatter-panel__title">Metadata</span>
              </div>
              <span className="page-frontmatter-panel__summary">
                {summaryParts.join(' • ')}
              </span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="page-frontmatter-panel__content">
            <div className="page-frontmatter-panel__tags-row">
              <Input
                value={tagDraft}
                onChange={(event) => setTagDraft(event.target.value)}
                onKeyDown={handleTagKeyDown}
                onBlur={() => {
                  commitTag(tagDraft)
                  setTagDraft('')
                }}
                placeholder="Add tag"
                className="page-frontmatter-panel__tag-input"
                data-testid="page-frontmatter-tag-input"
              />
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
              </div>
            </div>

            <div className="page-frontmatter-panel__section-heading">
              Properties
            </div>

            {fields.length > 0 ? (
              <div className="page-frontmatter-panel__fields">
                {fields.map((field, index) => (
                  <div
                    key={`${field.key}-${index}`}
                    className="page-frontmatter-panel__field-row"
                  >
                    <Input
                      value={field.key}
                      onChange={(event) =>
                        updateField(index, { key: event.target.value })
                      }
                      placeholder="Key"
                      className="page-frontmatter-panel__field-key"
                      data-testid={`page-frontmatter-field-key-${index}`}
                    />
                    <Select
                      value={field.type}
                      onValueChange={(value) =>
                        updateField(index, {
                          type: value as EditorFrontmatterFieldType,
                          value:
                            value === 'boolean'
                              ? 'true'
                              : value === 'list'
                                ? field.type === 'list'
                                  ? field.value
                                  : ''
                                : field.type === 'boolean'
                                  ? ''
                                  : field.value,
                        })
                      }
                      data-testid={`page-frontmatter-field-type-${index}`}
                    >
                      <SelectTrigger className="page-frontmatter-panel__field-select">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="text">Text</SelectItem>
                        <SelectItem value="number">Number</SelectItem>
                        <SelectItem value="boolean">Boolean</SelectItem>
                        <SelectItem value="list">List</SelectItem>
                      </SelectContent>
                    </Select>
                    <Input
                      value={getFieldValidation(field, fields, index)}
                      readOnly
                      className="page-frontmatter-panel__field-validation"
                      data-testid={`page-frontmatter-field-validation-${index}`}
                    />
                    {field.type === 'list' ? (
                      <textarea
                        value={field.value}
                        onChange={(event) =>
                          updateField(index, { value: event.target.value })
                        }
                        placeholder={valuePlaceholder(field.type)}
                        className="page-frontmatter-panel__field-value page-frontmatter-panel__field-value--list"
                        data-testid={`page-frontmatter-field-value-${index}`}
                      />
                    ) : field.type === 'boolean' ? (
                      <Select
                        value={field.value === 'false' ? 'false' : 'true'}
                        onValueChange={(value) => updateField(index, { value })}
                        data-testid={`page-frontmatter-field-value-${index}`}
                      >
                        <SelectTrigger className="page-frontmatter-panel__field-select">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="true">True</SelectItem>
                          <SelectItem value="false">False</SelectItem>
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input
                        type={field.type === 'number' ? 'number' : 'text'}
                        value={field.value}
                        onChange={(event) =>
                          updateField(index, { value: event.target.value })
                        }
                        placeholder={valuePlaceholder(field.type)}
                        className="page-frontmatter-panel__field-value"
                        data-testid={`page-frontmatter-field-value-${index}`}
                      />
                    )}
                    <button
                      type="button"
                      className="page-frontmatter-panel__field-remove"
                      onClick={() => removeField(index)}
                      aria-label={`Remove frontmatter field ${field.key || index + 1}`}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            ) : null}

            <div className="page-frontmatter-panel__actions">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addField}
                className="page-frontmatter-panel__add-button"
                data-testid="page-frontmatter-add-field"
              >
                <Plus size={14} />
                Add field
              </Button>
            </div>

            <p className="page-frontmatter-panel__hint">
              Keep fields flat for now. If you need nested metadata later, use
              dot keys like <code>seo.title</code>.
            </p>

            {hasUnsupportedFields ? (
              <p
                className="page-frontmatter-panel__notice"
                data-testid="page-frontmatter-unsupported-notice"
              >
                Existing advanced frontmatter is preserved in the background but
                not editable in this compact view yet.
              </p>
            ) : null}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </section>
  )
}
