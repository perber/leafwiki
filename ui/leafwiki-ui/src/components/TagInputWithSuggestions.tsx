import { fetchTags, TagCount } from '@/lib/api/tags'
import { Input } from '@/components/ui/input'
import { X } from 'lucide-react'
import {
  memo,
  KeyboardEvent,
  CompositionEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { createPortal } from 'react-dom'

type TagInputVariant = 'browse' | 'metadata'

type TagInputWithSuggestionsProps = {
  tags: string[]
  onTagsChange: (tags: string[]) => void
  placeholder: string
  variant: TagInputVariant
  inputTestId?: string
  inputHotkeys?: string
  active?: boolean
  onArrowDown?: () => boolean
  onArrowUp?: () => boolean
  onSubmitWithoutSuggestion?: () => void
}

function normalizeTags(tags: string[]) {
  const seen = new Set<string>()
  const result: string[] = []

  for (const rawTag of tags) {
    const normalized = rawTag.trim().toLocaleLowerCase()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    result.push(normalized)
  }

  return result
}

function variantClasses(variant: TagInputVariant) {
  if (variant === 'metadata') {
    return {
      root: 'page-frontmatter-panel__tag-picker',
      selection: 'page-frontmatter-panel__tags-inline',
      chip: 'page-frontmatter-panel__chip',
      chipRemove: 'page-frontmatter-panel__chip-remove',
      input: 'page-frontmatter-panel__tag-input',
      suggestions: 'page-frontmatter-panel__tag-suggestions',
      suggestion: 'page-frontmatter-panel__tag-suggestion',
      suggestionActive: 'page-frontmatter-panel__tag-suggestion--active',
      suggestionCount: 'page-frontmatter-panel__tag-suggestion-count',
      suggestionsEmpty: 'page-frontmatter-panel__tag-suggestions-empty',
    }
  }

  return {
    root: 'browse-tags__search-field',
    selection: 'browse-tags__search-selection',
    chip: 'browse-tags__selected-chip',
    chipRemove: 'browse-tags__selected-chip-remove',
    input: 'browse-tags__search-input browse-tags__search-input--plain',
    suggestions: 'browse-tags__suggestions',
    suggestion: 'browse-tags__suggestion',
    suggestionActive: 'browse-tags__suggestion--active',
    suggestionCount: 'browse-tags__suggestion-count',
    suggestionsEmpty: 'browse-tags__suggestions-empty',
  }
}

function allowsCustomTagCreation(variant: TagInputVariant) {
  return variant === 'metadata'
}

function usesSelectedTagSuggestions(variant: TagInputVariant) {
  return variant === 'browse'
}

function TagInputWithSuggestions({
  tags,
  onTagsChange,
  placeholder,
  variant,
  inputTestId,
  inputHotkeys,
  active = false,
  onArrowDown,
  onArrowUp,
  onSubmitWithoutSuggestion,
}: TagInputWithSuggestionsProps) {
  const { t } = useTranslation('common')
  const classes = variantClasses(variant)
  const allowCustomTags = allowsCustomTagCreation(variant)
  const useSelectedTagSuggestions = usesSelectedTagSuggestions(variant)
  const [draft, setDraft] = useState('')
  const [suggestions, setSuggestions] = useState<TagCount[]>([])
  const [loading, setLoading] = useState(false)
  const [suggestionsOpen, setSuggestionsOpen] = useState(false)
  const [activeSuggestionIndex, setActiveSuggestionIndex] = useState(0)
  const latestRequestIdRef = useRef(0)
  const filterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const commitDraftRef = useRef(true)
  const [dropdownStyle, setDropdownStyle] = useState<{
    top: number
    left: number
    width: number
  } | null>(null)

  const [isComposing, setIsComposing] = useState(false)

  const normalizedTags = useMemo(() => normalizeTags(tags), [tags])
  const suggestedTags = useMemo(
    () =>
      suggestions.filter(
        ({ tag }) => !normalizedTags.includes(tag.toLocaleLowerCase()),
      ),
    [normalizedTags, suggestions],
  )
  const showSuggestions = suggestionsOpen && draft.trim().length > 0
  const showNoSuggestions =
    showSuggestions && !loading && suggestedTags.length === 0
  const clampedSuggestionIndex =
    suggestedTags.length === 0
      ? 0
      : Math.min(activeSuggestionIndex, suggestedTags.length - 1)
  const activeSuggestion = suggestedTags[clampedSuggestionIndex]

  useEffect(() => {
    if (active) {
      inputRef.current?.focus()
    }
  }, [active])

  useEffect(() => {
    setActiveSuggestionIndex(0)
  }, [draft])

  useEffect(() => {
    if (suggestedTags.length === 0) {
      setActiveSuggestionIndex(0)
      return
    }
    setActiveSuggestionIndex((current) =>
      Math.min(current, suggestedTags.length - 1),
    )
  }, [suggestedTags.length])

  useEffect(() => {
    if (filterTimerRef.current) clearTimeout(filterTimerRef.current)
    const query = draft.trim().toLocaleLowerCase()

    if (!query || isComposing) {
      latestRequestIdRef.current += 1
      setSuggestions([])
      setLoading(false)
      return
    }

    filterTimerRef.current = setTimeout(async () => {
      const requestId = latestRequestIdRef.current + 1
      latestRequestIdRef.current = requestId
      setLoading(true)

      let isCurrentRequest = true
      try {
        const data = await fetchTags(
          query,
          20,
          useSelectedTagSuggestions ? normalizedTags : [],
        )
        isCurrentRequest = latestRequestIdRef.current === requestId
        if (!isCurrentRequest) return
        setSuggestions(data)
      } catch {
        isCurrentRequest = latestRequestIdRef.current === requestId
        if (!isCurrentRequest) return
        setSuggestions([])
      } finally {
        if (isCurrentRequest) {
          setLoading(false)
        }
      }
    }, 250)

    return () => {
      if (filterTimerRef.current) clearTimeout(filterTimerRef.current)
    }
  }, [draft, normalizedTags, isComposing, useSelectedTagSuggestions])

  useEffect(() => {
    if (!showSuggestions) {
      setDropdownStyle(null)
      return
    }
    const update = () => {
      const el = rootRef.current
      if (!el) return
      const rect = el.getBoundingClientRect()
      setDropdownStyle({
        top: rect.bottom + 4,
        left: rect.left,
        width: rect.width,
      })
    }
    update()
    window.addEventListener('scroll', update, true)
    window.addEventListener('resize', update)
    return () => {
      window.removeEventListener('scroll', update, true)
      window.removeEventListener('resize', update)
    }
  }, [showSuggestions])

  const addTag = (value: string) => {
    const nextTags = normalizeTags([...normalizedTags, value])
    onTagsChange(nextTags)
    setDraft('')
    setSuggestions([])
    setSuggestionsOpen(false)
    setActiveSuggestionIndex(0)
  }

  const removeTag = (value: string) => {
    const normalized = value.trim().toLocaleLowerCase()
    onTagsChange(normalizedTags.filter((tag) => tag !== normalized))
  }

  const commitDraft = () => {
    const normalized = draft.trim().toLocaleLowerCase()
    if (!normalized) return false
    if (normalizedTags.includes(normalized)) {
      setDraft('')
      return true
    }
    if (!allowCustomTags) {
      return false
    }
    addTag(normalized)
    return true
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    const inputValue = event.currentTarget.value
    const caretAtStart =
      event.currentTarget.selectionStart === 0 &&
      event.currentTarget.selectionEnd === 0

    if (
      event.key === 'Backspace' &&
      inputValue.length === 0 &&
      caretAtStart &&
      normalizedTags.length > 0
    ) {
      event.preventDefault()
      onTagsChange(normalizedTags.slice(0, -1))
      setSuggestionsOpen(false)
      return
    }

    if (event.key === 'ArrowDown') {
      if (showSuggestions && suggestedTags.length > 0) {
        event.preventDefault()
        setActiveSuggestionIndex((current) =>
          Math.min(current + 1, suggestedTags.length - 1),
        )
        return
      }

      if (onArrowDown?.()) {
        event.preventDefault()
        setSuggestionsOpen(false)
        return
      }
    }

    if (event.key === 'ArrowUp') {
      if (showSuggestions && suggestedTags.length > 0) {
        event.preventDefault()
        setActiveSuggestionIndex((current) => Math.max(current - 1, 0))
        return
      }

      if (onArrowUp?.()) {
        event.preventDefault()
        setSuggestionsOpen(false)
        return
      }
    }

    if (event.key !== 'Enter' && event.key !== ',') {
      return
    }

    if (showSuggestions && activeSuggestion) {
      event.preventDefault()
      addTag(activeSuggestion.tag)
      return
    }

    if (event.key === 'Enter' && !draft.trim()) {
      if (onSubmitWithoutSuggestion) {
        event.preventDefault()
        onSubmitWithoutSuggestion()
      }
      return
    }

    if (commitDraft()) {
      event.preventDefault()
      return
    }

    if (!allowCustomTags && (event.key === 'Enter' || event.key === ',')) {
      event.preventDefault()
    }
  }

  const suggestionsContent =
    showSuggestions && dropdownStyle
      ? createPortal(
          <div
            className={classes.suggestions}
            style={{
              position: 'fixed',
              top: dropdownStyle.top,
              left: dropdownStyle.left,
              width: dropdownStyle.width,
              zIndex: 50,
            }}
          >
            {loading ? (
              <p className={classes.suggestionsEmpty}>{t('tags.loading')}</p>
            ) : showNoSuggestions ? (
              <p className={classes.suggestionsEmpty}>
                {t('tags.noMatches')}
              </p>
            ) : (
              suggestedTags.map(({ tag }, index) => (
                <button
                  key={tag}
                  type="button"
                  className={`${classes.suggestion}${index === clampedSuggestionIndex ? ` ${classes.suggestionActive}` : ''}`}
                  onMouseDown={(event) => {
                    event.preventDefault()
                    commitDraftRef.current = false
                  }}
                  onClick={() => addTag(tag)}
                  onMouseEnter={() => setActiveSuggestionIndex(index)}
                  aria-selected={index === clampedSuggestionIndex}
                  data-testid={
                    variant === 'browse'
                      ? `tags-suggestion-${tag}`
                      : `tag-suggestion-${variant}-${tag}`
                  }
                >
                  <span>{tag}</span>
                </button>
              ))
            )}
          </div>,
          document.body,
        )
      : null

  return (
    <div ref={rootRef} className={classes.root}>
      <div className={classes.selection}>
        {normalizedTags.map((tag) => (
          <span
            key={tag}
            className={classes.chip}
            data-testid={
              variant === 'browse' ? `tags-selected-chip-${tag}` : undefined
            }
          >
            <span>{tag}</span>
            <button
              type="button"
              className={classes.chipRemove}
              onClick={() => removeTag(tag)}
              aria-label={t('tags.remove', { tag })}
            >
              <X size={12} />
            </button>
          </span>
        ))}
        <Input
          ref={inputRef}
          value={draft}
          onChange={(event) => {
            setDraft(event.target.value)
            setSuggestionsOpen(true)
          }}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={(event: CompositionEvent<HTMLInputElement>) => {
            setIsComposing(false)
            setDraft(event.currentTarget.value)
          }}
          onKeyDown={handleKeyDown}
          onFocus={() => setSuggestionsOpen(true)}
          onBlur={() => {
            window.setTimeout(() => {
              setSuggestionsOpen(false)
              if (commitDraftRef.current) {
                commitDraft()
              }
              commitDraftRef.current = true
            }, 100)
          }}
          placeholder={placeholder}
          className={classes.input}
          data-testid={inputTestId}
          data-allow-hotkeys={inputHotkeys}
        />
      </div>

      {suggestionsContent}
    </div>
  )
}

export default memo(TagInputWithSuggestions)
