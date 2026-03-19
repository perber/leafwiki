import { PageRefactorPreview } from '@/lib/api/pages'
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

export async function refreshAfterPageRefactor({
  preview,
  currentPath,
  navigate,
}: RefreshAfterPageRefactorOptions) {
  await useTreeStore.getState().reloadTree()

  const currentViewerPage = useViewerStore.getState().page
  const normalizedCurrentPath = normalizeRoutePath(
    currentViewerPage?.path || '',
  )
  const normalizedRoutePath = normalizeRoutePath(currentPath)
  const isViewingMovedPage =
    normalizedCurrentPath === preview.oldPath ||
    normalizedCurrentPath === preview.newPath ||
    normalizedRoutePath === preview.oldPath ||
    normalizedRoutePath === preview.newPath

  let nextPath: string | null = null

  if (isViewingMovedPage) {
    nextPath = preview.newPath
    if (normalizedRoutePath !== preview.newPath) {
      navigate(preview.newPath, { replace: true })
    }
  } else if (currentViewerPage) {
    nextPath = normalizedCurrentPath
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
