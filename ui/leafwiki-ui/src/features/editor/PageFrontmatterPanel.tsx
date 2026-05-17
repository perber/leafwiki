import TagInputWithSuggestions from '@/components/TagInputWithSuggestions'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ChevronDown, ChevronRight, Plus, Tag, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { EditorFrontmatterField } from './frontmatter'

type PageFrontmatterPanelProps = {
  tags: string[]
  fields: EditorFrontmatterField[]
  errors: Record<string, string>
  hasUnsupportedFields: boolean
  onTagsChange: (tags: string[]) => void
  onFieldsChange: (fields: EditorFrontmatterField[]) => void
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
  errors,
  hasUnsupportedFields,
  onTagsChange,
  onFieldsChange,
}: PageFrontmatterPanelProps) {
  const [showInternalFields, setShowInternalFields] = useState(false)

  const normalizedTags = useMemo(() => {
    const seen = new Set<string>()
    return tags.filter((tag) => {
      const normalized = tag.trim().toLocaleLowerCase()
      if (!normalized) return false
      if (seen.has(normalized)) return false
      seen.add(normalized)
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

  const hasErrors = Object.keys(errors).length > 0

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
          <AccordionTrigger
            className={`page-frontmatter-panel__trigger${hasErrors ? 'page-frontmatter-panel__trigger--has-errors' : ''}`}
          >
            <div className="page-frontmatter-panel__topline">
              <div
                className={`page-frontmatter-panel__title-row${hasErrors ? 'page-frontmatter-panel__title-row--has-errors' : ''}`}
              >
                <Tag className="page-frontmatter-panel__title-icon" size={14} />
                <span className="page-frontmatter-panel__title">Metadata</span>
              </div>
              <span
                className={`page-frontmatter-panel__summary${hasErrors ? 'page-frontmatter-panel__summary--has-errors' : ''}`}
              >
                {summaryParts.join(' • ')}
              </span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="page-frontmatter-panel__content">
            <div className="page-frontmatter-panel__stack">
              <div className="page-frontmatter-panel__row page-frontmatter-panel__row--tags">
                <div className="page-frontmatter-panel__section-heading page-frontmatter-panel__section-heading--inline">
                  Tags
                </div>
                <div className="page-frontmatter-panel__tags-field">
                  <TagInputWithSuggestions
                    tags={normalizedTags}
                    onTagsChange={onTagsChange}
                    placeholder="Add tag"
                    variant="metadata"
                    inputTestId="page-frontmatter-tag-input"
                  />
                  {errors.tags ? (
                    <p
                      className="page-frontmatter-panel__error"
                      data-testid="page-frontmatter-tags-error"
                    >
                      {errors.tags}
                    </p>
                  ) : null}
                </div>
              </div>

              <div className="page-frontmatter-panel__row page-frontmatter-panel__row--properties">
                <div className="page-frontmatter-panel__section-heading page-frontmatter-panel__section-heading--inline">
                  Properties
                </div>
                <div className="page-frontmatter-panel__properties">
                  <div className="page-frontmatter-panel__properties-scroll custom-scrollbar">
                    {editableFields.length > 0 ? (
                      <div className="page-frontmatter-panel__fields">
                        {editableFields.map((field, index) => (
                          <div key={`editable-field-${index}`}>
                            <div className="page-frontmatter-panel__field-row">
                              <Input
                                value={field.key}
                                onChange={(event) =>
                                  updateField(index, {
                                    key: event.target.value,
                                  })
                                }
                                placeholder="Key"
                                className={`page-frontmatter-panel__field-key${errors[`properties.${index}.key`] ? 'page-frontmatter-panel__input--error' : ''}`}
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
                                className={`page-frontmatter-panel__field-value${errors[`properties.${index}.value`] ? 'page-frontmatter-panel__input--error' : ''}`}
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
                            {errors[`properties.${index}.key`] ? (
                              <p
                                className="page-frontmatter-panel__error"
                                data-testid={`page-frontmatter-field-key-error-${index}`}
                              >
                                {errors[`properties.${index}.key`]}
                              </p>
                            ) : null}
                            {errors[`properties.${index}.value`] ? (
                              <p
                                className="page-frontmatter-panel__error"
                                data-testid={`page-frontmatter-field-value-error-${index}`}
                              >
                                {errors[`properties.${index}.value`]}
                              </p>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    ) : null}

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
                  </div>

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
