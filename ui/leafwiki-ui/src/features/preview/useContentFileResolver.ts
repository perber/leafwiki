import { buildViewUrl, stripBasePath, withBasePath } from '@/lib/routePath'
import {
  normalizeWikiRoutePath,
  resolveContentFilePath,
  toWikiLookupPath,
  type RelativeLinkMode,
} from '@/lib/wikiPath'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { useCallback } from 'react'

/**
 * Media references that are candidates for content-file resolution.
 * Uploaded page attachments (/assets), API URLs and absolute web/data
 * sources are handled by their own mechanisms and pass through unchanged.
 */
function isResolvableContentSrc(src: string): boolean {
  if (!src) return false
  if (/^(https?:|data:|blob:|mailto:|#|\/\/)/i.test(src)) return false
  if (src.startsWith('/assets') || src.startsWith('assets/')) return false
  if (src.startsWith('/api/')) return false
  if (src.startsWith('/files/')) return false
  return true
}

/**
 * Resolves relative Markdown media srcs (images, videos, PDFs — e.g.
 * `img/shot.png`) against the current page's directory, exactly like the
 * page's relative links resolve (see resolveWikiLinkPath), and maps the
 * result onto the backend `/files` route that serves media straight from
 * the wiki content tree. Absolute and asset srcs pass through unchanged.
 */
export function useContentFileResolver(): (
  src: string,
  path?: string,
) => string {
  const relativeLinkMode = useConfigStore(
    (s) => s.relativeLinkMode,
  ) as RelativeLinkMode
  const getPageByPath = useTreeStore((s) => s.getPageByPath)

  return useCallback(
    (src: string, path?: string) => {
      if (!isResolvableContentSrc(src)) return src

      let locationPath = window.location.pathname
      const stripped = stripBasePath(locationPath)
      if (stripped !== null) {
        locationPath = stripped
      }
      const currentWikiPath = normalizeWikiRoutePath(
        path ?? buildViewUrl(locationPath),
      )
      const currentIsSection =
        getPageByPath(toWikiLookupPath(currentWikiPath))?.kind === 'section'
      const resolved = resolveContentFilePath(
        currentWikiPath,
        src,
        relativeLinkMode,
        currentIsSection,
      )
      return withBasePath(`/files${resolved}`)
    },
    [relativeLinkMode, getPageByPath],
  )
}

const MEDIA_EXTS = new Set([
  'png', 'jpg', 'jpeg', 'gif', 'webp', 'avif', 'bmp', 'svg', 'ico',
  'pdf',
  'mp4', 'webm', 'ogv', 'mov', 'm4v',
  'mp3', 'wav', 'ogg', 'oga', 'm4a', 'flac', 'aac', 'opus',
])

/**
 * Reports whether a resolved wiki route addresses a media file served by
 * the backend /files route (mirrors serveContentExts in
 * internal/wiki/contentfiles/routes.go) rather than a wiki page.
 */
export function isContentFileRoutePath(path: string): boolean {
  const dot = path.lastIndexOf('.')
  if (dot === -1) return false
  const ext = path.slice(dot + 1).toLowerCase()
  if (!MEDIA_EXTS.has(ext)) return false
  // A hash/query would already be stripped by normalizeWikiRoutePath; a
  // path with no slash after the dot is not a file name.
  return path.indexOf('/', dot) === -1
}
