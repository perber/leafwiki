import { PageRefactorPreview } from '@/lib/api/pages'
import { buildEditUrl, buildHistoryUrl, buildViewUrl } from '@/lib/routePath'
import { normalizeWikiRoutePath } from '@/lib/wikiPath'
import { useTreeStore } from '@/stores/tree'
import { NavigateFunction } from 'react-router-dom'
import { useLinkStatusStore } from '../links/linkstatus_store'
import { useViewerStore } from '../viewer/viewer'

type RefreshAfterPageRefactorOptions = {
  preview: PageRefactorPreview
  currentPath: string
  navigate: NavigateFunction
}

function normalizeRoutePath(path: string) {
  if (!path) {
    return '/'
  }
  return path.startsWith('/') ? path : `/${path}`
}

function toPageLookupPath(path: string) {
  return normalizeRoutePath(path).replace(/^\/+/, '')
}

function buildRefactorRoutePath(currentPath: string, nextWikiPath: string) {
  const normalizedCurrentPath = normalizeRoutePath(currentPath)

  if (
    normalizedCurrentPath === '/history' ||
    normalizedCurrentPath === '/history/'
  ) {
    return buildHistoryUrl(nextWikiPath)
  }
  if (normalizedCurrentPath.startsWith('/history/')) {
    return buildHistoryUrl(nextWikiPath)
  }
  if (normalizedCurrentPath.startsWith('/e/')) {
    return buildEditUrl(nextWikiPath)
  }

  return buildViewUrl(nextWikiPath)
}

export async function refreshAfterPageRefactor({
  preview,
  currentPath,
  navigate,
}: RefreshAfterPageRefactorOptions) {
  await useTreeStore.getState().reloadTree()

  const currentViewerPage = useViewerStore.getState().page
  const normalizedViewerPath = normalizeWikiRoutePath(
    currentViewerPage?.path || '',
  )
  const normalizedRoutePath = normalizeWikiRoutePath(buildViewUrl(currentPath))
  const normalizedOldPath = normalizeWikiRoutePath(preview.oldPath)
  const normalizedNewPath = normalizeWikiRoutePath(preview.newPath)
  const isViewingMovedPage =
    normalizedViewerPath === normalizedOldPath ||
    normalizedViewerPath === normalizedNewPath ||
    normalizedRoutePath === normalizedOldPath ||
    normalizedRoutePath === normalizedNewPath

  let nextPath: string | null = null

  if (isViewingMovedPage) {
    nextPath = preview.newPath
    const nextRoutePath = buildRefactorRoutePath(currentPath, preview.newPath)
    if (normalizeRoutePath(currentPath) !== nextRoutePath) {
      navigate(nextRoutePath, { replace: true })
    }
  } else if (currentViewerPage) {
    nextPath = normalizedViewerPath
  }

  if (!nextPath) {
    return
  }

  await useViewerStore.getState().loadPageData(toPageLookupPath(nextPath))

  const viewerPageID = useViewerStore.getState().page?.id
  if (!viewerPageID) {
    useLinkStatusStore.getState().clear()
    return
  }

  await useLinkStatusStore.getState().fetchLinkStatusForPage(viewerPageID)
}
