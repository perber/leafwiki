import { FormInput } from '@/components/FormInput'
import { suggestSlug } from '@/lib/api/pages'
import { useDebounce } from '@/lib/useDebounce'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

type Props = {
  title: string
  slug: string
  parentId: string
  testid?: string
  currentId?: string
  enableSlugSuggestion?: boolean
  onSlugChange: (slug: string) => void
  onSlugTouchedChange?: (touched: boolean) => void
  onSlugLoadingChange?: (loading: boolean) => void
  onLastSlugTitleChange?: (title: string) => void
  error?: string
}

export function SlugInputWithSuggestion({
  title,
  slug,
  currentId,
  testid,
  parentId,
  enableSlugSuggestion = true,
  onSlugChange,
  onSlugTouchedChange,
  onSlugLoadingChange,
  onLastSlugTitleChange,
  error,
}: Props) {
  const [slugTouched, setSlugTouched] = useState(false)
  const debouncedTitle = useDebounce(title, 300)

  useEffect(() => {
    if (!enableSlugSuggestion || slugTouched || debouncedTitle.trim() === '') {
      return
    }

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
      } catch {
        toast.error('Error generating slug')
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
      label="Slug"
      value={slug}
      onChange={handleChange}
      placeholder="Page slug"
      testid={testid}
      error={error}
    />
  )
}
