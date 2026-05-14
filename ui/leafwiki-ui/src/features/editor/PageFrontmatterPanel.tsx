import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ChevronDown, ChevronRight, Plus, Tag, Trash2, X } from 'lucide-react'
import { KeyboardEvent, useMemo, useState } from 'react'
import { EditorFrontmatterField } from './frontmatter'

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

function getFieldValue(field: EditorFrontmatterField) {
  if (field.type === 'list') {
    return field.value
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)
      .join(', ')
  }

  return field.value
}

export function PageFrontmatterPanel({
  tags,
  fields,
  hasUnsupportedFields,
  onTagsChange,
  onFieldsChange,
}: PageFrontmatterPanelProps) {
  const [tagDraft, setTagDraft] = useState('')
  const [showInternalFields, setShowInternalFields] = useState(false)

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

  const editableFields = useMemo(
    () => fields.filter((field) => !field.internal),
    [fields],
  )

  const internalFields = useMemo(
    () => fields.filter((field) => field.internal),
    [fields],
  )

  const mergeEditableFields = (
    nextEditableFields: EditorFrontmatterField[],
  ) => {
    const merged: EditorFrontmatterField[] = []
    let editableIndex = 0

    for (const field of fields) {
      if (field.internal) {
        merged.push(field)
        continue
      }

      if (editableIndex < nextEditableFields.length) {
        merged.push(nextEditableFields[editableIndex])
        editableIndex += 1
      }
    }

    while (editableIndex < nextEditableFields.length) {
      merged.push(nextEditableFields[editableIndex])
      editableIndex += 1
    }

    onFieldsChange(merged)
  }

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
    const next = editableFields.map((field, currentIndex) =>
      currentIndex === index ? { ...field, ...patch } : field,
    )
    mergeEditableFields(next)
  }

  const removeField = (index: number) => {
    mergeEditableFields(
      editableFields.filter((_, currentIndex) => currentIndex !== index),
    )
  }

  const addField = () => {
    mergeEditableFields([...editableFields, buildEmptyField()])
  }

  const summaryParts = [
    normalizedTags.length === 1 ? '1 tag' : `${normalizedTags.length} tags`,
    editableFields.length === 1
      ? '1 property'
      : `${editableFields.length} properties`,
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
            <div className="page-frontmatter-panel__layout">
              <div className="page-frontmatter-panel__group page-frontmatter-panel__group--tags">
                <div className="page-frontmatter-panel__section-heading">
                  Tags
                </div>
                <div className="page-frontmatter-panel__tags-field">
                  <div className="page-frontmatter-panel__tags-inline">
                    {normalizedTags.map((tag) => (
                      <span key={tag} className="page-frontmatter-panel__chip">
                        <span>{tag}</span>
                        <button
                          type="button"
                          className="page-frontmatter-panel__chip-remove"
                          onClick={() =>
                            onTagsChange(
                              normalizedTags.filter(
                                (current) => current !== tag,
                              ),
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
                      placeholder="Add tag"
                      className="page-frontmatter-panel__tag-input"
                      data-testid="page-frontmatter-tag-input"
                    />
                  </div>
                </div>
              </div>

              <div className="page-frontmatter-panel__group page-frontmatter-panel__group--properties">
                <div className="page-frontmatter-panel__section-heading">
                  Properties
                </div>
                <div className="page-frontmatter-panel__properties-scroll custom-scrollbar">
                  {editableFields.length > 0 ? (
                    <div className="page-frontmatter-panel__fields">
                      {editableFields.map((field, index) => (
                        <div
                          key={`editable-field-${index}`}
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
                          <Input
                            type="text"
                            value={getFieldValue(field)}
                            onChange={(event) =>
                              updateField(index, {
                                type: 'text',
                                value: event.target.value,
                              })
                            }
                            placeholder="Value"
                            className="page-frontmatter-panel__field-value"
                            data-testid={`page-frontmatter-field-value-${index}`}
                          />
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
                      Add property
                    </Button>
                  </div>

                  {internalFields.length > 0 ? (
                    <div className="page-frontmatter-panel__internal">
                      <button
                        type="button"
                        className="page-frontmatter-panel__internal-toggle"
                        onClick={() =>
                          setShowInternalFields((current) => !current)
                        }
                        data-testid="page-frontmatter-internal-toggle"
                      >
                        {showInternalFields ? (
                          <ChevronDown size={14} />
                        ) : (
                          <ChevronRight size={14} />
                        )}
                        Internal fields
                      </button>

                      {showInternalFields ? (
                        <div className="page-frontmatter-panel__fields page-frontmatter-panel__fields--internal">
                          {internalFields.map((field, index) => (
                            <div
                              key={`internal-field-${index}`}
                              className="page-frontmatter-panel__field-row"
                            >
                              <Input
                                value={field.key}
                                readOnly
                                className="page-frontmatter-panel__field-key page-frontmatter-panel__field-key--readonly"
                              />
                              <Input
                                type="text"
                                value={getFieldValue(field)}
                                readOnly
                                className="page-frontmatter-panel__field-value page-frontmatter-panel__field-value--readonly"
                              />
                              <span className="page-frontmatter-panel__field-spacer" />
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ) : null}

                  <p className="page-frontmatter-panel__hint">
                    Keep fields flat for now. If you need nested metadata later,
                    use dot keys like <code>seo.title</code>.
                  </p>

                  {hasUnsupportedFields ? (
                    <p
                      className="page-frontmatter-panel__notice"
                      data-testid="page-frontmatter-unsupported-notice"
                    >
                      Existing advanced frontmatter is preserved in the
                      background but not editable in this compact view yet.
                    </p>
                  ) : null}
                </div>
              </div>
            </div>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </section>
  )
}
