import { fetchWithAuth } from './auth'

export const NODE_KIND_PAGE = 'page'
export const NODE_KIND_SECTION = 'section'

export type PageMetadata = {
  createdAt: string
  updatedAt: string
  creatorId: string
  lastAuthorId: string
  creator?: {
    id: string
    username: string
  }
  lastAuthor?: {
    id: string
    username: string
  }
}

export type PageNode = {
  id: string
  title: string
  slug: string
  path: string
  parentId?: string | null
  children: PageNode[] | null
  kind: 'page' | 'section'
  metadata?: PageMetadata // optional metadata, because older API responses may not have it
}

export interface Page {
  id: string
  slug: string
  path: string
  title: string
  content: string
  kind: 'page' | 'section'
  metadata?: PageMetadata // optional metadata, because older API responses may not have it
}

export async function fetchTree(): Promise<PageNode> {
  return (await fetchWithAuth(`/api/tree`)) as PageNode
}

export async function suggestSlug(
  parentId: string,
  title: string,
  currentId?: string,
): Promise<string> {
  try {
    if (!currentId) currentId = ''

    const data = await fetchWithAuth(
      `/api/pages/slug-suggestion?parentID=${parentId}&title=${encodeURIComponent(title)}${currentId ? `&currentID=${currentId}` : ''}`,
    )
    const typedData = data as { slug: string }
    return typedData.slug
  } catch {
    throw new Error('Slug suggestion failed')
  }
}

export async function getPageByPath(path: string): Promise<Page> {
  try {
    return (await fetchWithAuth(
      `/api/pages/by-path?path=${encodeURIComponent(path)}`,
    )) as Page
  } catch {
    throw new Error('Page not found')
  }
}

export async function createPage({
  title,
  slug,
  parentId,
  kind,
}: {
  title: string
  slug: string
  parentId: string | null
  kind: 'page' | 'section'
}) {
  if (parentId === '') parentId = null

  console.log('Creating page with kind:', kind)

  return await fetchWithAuth(`/api/pages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, slug, parentId, kind }),
  })
}

export async function copyPage(
  id: string,
  targetParentId: string | null,
  targetTitle: string,
  targetSlug: string,
) {
  if (targetParentId === '' || targetParentId === 'root') targetParentId = null
  return await fetchWithAuth(`/api/pages/copy/${id}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      targetParentId,
      title: targetTitle,
      slug: targetSlug,
    }),
  })
}

export async function updatePage(
  id: string,
  title: string,
  slug: string,
  content: string,
): Promise<Page | null> {
  return (await fetchWithAuth(`/api/pages/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, slug, content }),
  })) as Page | null
}

export async function deletePage(id: string, recursive: boolean) {
  if (recursive === undefined) recursive = false

  const recursiveQuery = recursive ? 'true' : 'false'

  return await fetchWithAuth(`/api/pages/${id}?recursive=${recursiveQuery}`, {
    method: 'DELETE',
  })
}

export async function movePage(id: string, parentId: string | null) {
  if (parentId === '' || parentId == 'root') parentId = null

  return await fetchWithAuth(`/api/pages/${id}/move`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parentId }),
  })
}

export async function sortPages(parentId: string, orderedIDs: string[]) {
  if (parentId === '') parentId = 'root'

  return await fetchWithAuth(`/api/pages/${parentId}/sort`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ orderedIDs }),
  })
}

export async function convertPage(id: string, targetKind: 'page' | 'section') {
  return await fetchWithAuth(`/api/pages/convert/${id}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ targetKind }),
  })
}

export type PathLookupResult = {
  path: string
  exists: boolean
  segments: { slug: string; id?: string; exists: boolean }[]
}

export async function lookupPath(path: string): Promise<PathLookupResult> {
  return (await fetchWithAuth(
    `/api/pages/lookup?path=${encodeURIComponent(path)}`,
  )) as {
    path: string
    exists: boolean
    segments: { slug: string; id?: string; exists: boolean }[]
  }
}

export async function ensurePage(path: string, targetTitle: string) {
  return await fetchWithAuth(`/api/pages/ensure`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, targetTitle }),
  })
}
