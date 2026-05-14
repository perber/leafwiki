import { create } from 'zustand'

function normalizeTag(tag: string) {
  return tag.trim().toLocaleLowerCase()
}

function hasTag(tags: string[], tag: string) {
  const normalized = normalizeTag(tag)
  return tags.some((current) => normalizeTag(current) === normalized)
}

type TagsStore = {
  activeTags: string[]
  setActiveTags: (tags: string[]) => void
  addActiveTag: (tag: string) => void
  removeActiveTag: (tag: string) => void
  clearActiveTags: () => void
  toggleActiveTag: (tag: string) => void
}

export const useTagsStore = create<TagsStore>()((set) => ({
  activeTags: [],
  setActiveTags: (tags) =>
    set({
      activeTags: tags
        .map(normalizeTag)
        .filter((tag, index, all) => tag && all.indexOf(tag) === index),
    }),
  addActiveTag: (tag) =>
    set((state) => {
      const normalized = normalizeTag(tag)
      if (!normalized || hasTag(state.activeTags, normalized)) return state
      return { activeTags: [...state.activeTags, normalized] }
    }),
  removeActiveTag: (tag) =>
    set((state) => ({
      activeTags: state.activeTags.filter(
        (current) => normalizeTag(current) !== normalizeTag(tag),
      ),
    })),
  clearActiveTags: () => set({ activeTags: [] }),
  toggleActiveTag: (tag) =>
    set((state) => ({
      activeTags: hasTag(state.activeTags, tag)
        ? state.activeTags.filter(
            (current) => normalizeTag(current) !== normalizeTag(tag),
          )
        : [...state.activeTags, normalizeTag(tag)].filter(Boolean),
    })),
}))
