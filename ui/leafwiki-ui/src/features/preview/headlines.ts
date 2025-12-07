import { create } from 'zustand'

interface HeadlinesState {
  headlines: { level: number; text: string; slug: string; id: string }[]
  registerHeadline: (level: number, text: string, id: string) => void
  unregisterHeadline: (level: number, text: string, id: string) => void
  getSlug: (level: number, text: string, id: string) => string | undefined
  clearHeadlines: () => void
}

// Utility function to generate slugs from headline text
const slugify = (text: string) => {
  // replace special characters like ä, ö, ü, ß etc. with a, o, u, s
  const specialChars = {
    ö: 'o',
    ü: 'u',
    ß: 's',
    ä: 'a',
  }

  return text
    .toLowerCase()
    .replace(
      /[öüßä]/g,
      (char) => specialChars[char as keyof typeof specialChars],
    )
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/[\s_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

export const useHeadlinesStore = create<HeadlinesState>((set, get) => ({
  headlines: [],
  getSlug: (level, text, id) => {
    return get().headlines.find(
      (h) => h.level === level && h.text === text && h.id === id,
    )?.slug
  },
  registerHeadline: (level, text, id) => {
    const existingHeadlines = get().headlines
    // Remove any existing headline with the same level, text, and id to prevent duplicates
    const filteredHeadlines = existingHeadlines.filter(
      (h) => !(h.level === level && h.text === text && h.id === id),
    )
    // the headline needs to be suffixed in the same order as displayed on the page
    // therefore we need to order the headlines based on their data-line id
    // we need to update all slugs after the inserted headline to ensure uniqueness

    const sortedHeadlines = [
      ...filteredHeadlines,
      { level, text, id, slug: '' },
    ].sort((a, b) => parseInt(a.id) - parseInt(b.id))
    const slugCounts: Record<string, number> = {}
    const uniqueSlugs: string[] = []

    for (const headline of sortedHeadlines) {
      const baseSlug = slugify(headline.text)
      let uniqueSlug = baseSlug

      if (slugCounts[baseSlug] !== undefined) {
        slugCounts[baseSlug] += 1
        uniqueSlug = `${baseSlug}-${slugCounts[baseSlug]}`
      } else {
        slugCounts[baseSlug] = 0
      }

      uniqueSlugs.push(uniqueSlug)
    }

    const updatedHeadlines = sortedHeadlines.map((h, i) => ({
      ...h,
      slug: uniqueSlugs[i],
    }))

    set({
      headlines: updatedHeadlines,
    })
  },
  unregisterHeadline: (level, text, id) => {
    const existingHeadlines = get().headlines
    set({
      headlines: existingHeadlines.filter(
        (h) => h.level !== level || h.text !== text || h.id !== id,
      ),
    })
  },
  clearHeadlines: () => {
    set({ headlines: [] })
  },
}))
