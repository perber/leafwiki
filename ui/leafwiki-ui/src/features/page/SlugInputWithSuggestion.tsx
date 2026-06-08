import { FormInput } from '@/components/FormInput'
import { mapApiError } from '@/lib/api/errors'
import { suggestSlug } from '@/lib/api/pages'
import i18next from '@/lib/i18n'
import { useDebounce } from '@/lib/useDebounce'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

type Props = {
  title: string
  slug: string
  parentId: string
  testid?: string
  currentId?: string
  /**
   * When provided, the slug suggestion API is skipped on the first render if
   * the title still matches this value. This preserves the existing slug (e.g.
   * "grafana-1") when opening the edit dialog for a page whose slug was
   * disambiguated by the server.
   */
  initialTitle?: string
  enableSlugSuggestion?: boolean
  onSlugChange: (slug: string) => void
  onSlugTouchedChange?: (touched: boolean) => void
  onSlugLoadingChange?: (loading: boolean) => void
  onLastSlugTitleChange?: (title: string) => void
  error?: string
  allowedHotkeys?: string
}

export function SlugInputWithSuggestion({
  title,
  slug,
  currentId,
  testid,
  parentId,
  initialTitle,
  enableSlugSuggestion = true,
  onSlugChange,
  onSlugTouchedChange,
  onSlugLoadingChange,
  onLastSlugTitleChange,
  error,
  allowedHotkeys,
}: Props) {
  const [slugTouched, setSlugTouched] = useState(false)
  const debouncedTitle = useDebounce(title, 300)
  // Tracks whether we have already processed the initial unchanged-title case.
  const initialHandledRef = useRef(false)

  useEffect(() => {
    if (!enableSlugSuggestion || slugTouched || debouncedTitle.trim() === '') {
      return
    }

    // If the title hasn't changed from its initial value on the first run,
    // preserve the existing slug instead of overwriting it with a fresh
    // suggestion. This prevents "grafana-1" from being replaced by "grafana"
    // when opening the edit dialog without modifying the title.
    if (
      !initialHandledRef.current &&
      initialTitle !== undefined &&
      debouncedTitle === initialTitle
    ) {
      initialHandledRef.current = true
      onLastSlugTitleChange?.(debouncedTitle)
      return
    }
    initialHandledRef.current = true

    const generateSlug = async () => {
      try {
        onSlugLoadingChange?.(true)
        const suggestion = await suggestSlug(
          parentId,
          debouncedTitle,
          currentId,
        )
        onSlugChange(suggestion)
        onLastSlugTitleChange?.(debouncedTitle)
      } catch (err) {
        const mapped = mapApiError(err, 'Error generating slug')
        toast.error(mapped.message)
      } finally {
        onSlugLoadingChange?.(false)
      }
    }

    generateSlug()
  }, [
    debouncedTitle,
    slugTouched,
    parentId,
    currentId,
    initialTitle,
    onSlugLoadingChange,
    onSlugChange,
    onLastSlugTitleChange,
    enableSlugSuggestion,
  ])

  const handleChange = (val: string) => {
    onSlugChange(val)
    setSlugTouched(true)
    onSlugTouchedChange?.(true)
  }

  return (
    <FormInput
      label={i18next.t('slugInput.label', { ns: 'editor' })}
      value={slug}
      onChange={handleChange}
      placeholder={i18next.t('slugInput.placeholder', { ns: 'editor' })}
      testid={testid}
      error={error}
      allowedHotkeys={allowedHotkeys}
    />
  )
}
