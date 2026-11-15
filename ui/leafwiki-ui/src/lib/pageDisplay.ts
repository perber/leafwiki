import {
  DEFAULT_CONTENT_WIDTH,
  toContentWidth,
  type ContentWidth,
} from '@/lib/contentWidth'

// The instance-wide page display settings (Settings > Display). Mirrors
// pagedisplay.Settings in internal/pagedisplay/provider.go: the field names
// are the ones GET /api/config and PUT /api/admin/settings/display use. A new
// setting is a new field here and in parsePageDisplay.
export type PageDisplay = {
  // Show the TOC panel/dropdown regardless of heading count.
  alwaysShowToc: boolean
  // Page reading-column width.
  contentWidth: ContentWidth
  // Show how many direct subpages a section has (tree, section overview).
  showSubpageCount: boolean
}

// The fields of a PUT /api/admin/settings/display request; a missing field
// is left unchanged by the server.
export type PageDisplayPatch = Partial<PageDisplay>

export const DEFAULT_PAGE_DISPLAY: PageDisplay = {
  alwaysShowToc: false,
  contentWidth: DEFAULT_CONTENT_WIDTH,
  showSubpageCount: false,
}

// Reads the display settings from a server response (/api/config or the
// display PUT reply). A missing field (older server) or a value of the wrong
// type falls back to its default.
export function parsePageDisplay(raw: unknown): PageDisplay {
  const fields: Record<string, unknown> =
    typeof raw === 'object' && raw !== null && !Array.isArray(raw)
      ? (raw as Record<string, unknown>)
      : {}
  return {
    alwaysShowToc:
      typeof fields.alwaysShowToc === 'boolean'
        ? fields.alwaysShowToc
        : DEFAULT_PAGE_DISPLAY.alwaysShowToc,
    contentWidth: toContentWidth(fields.contentWidth),
    showSubpageCount:
      typeof fields.showSubpageCount === 'boolean'
        ? fields.showSubpageCount
        : DEFAULT_PAGE_DISPLAY.showSubpageCount,
  }
}
