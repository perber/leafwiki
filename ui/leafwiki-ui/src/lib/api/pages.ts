import { fetchWithAuth } from './auth'

export type PageNode = {
  id: string
  title: string
  slug: string
  path: string
  parentId?: string | null
  children: PageNode[]
}

export interface Page {
  id: string
  path: string
  title: string
  content: string
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

export async function getPageByPath(path: string) {
  try {
    return await fetchWithAuth(
      `/api/pages/by-path?path=${encodeURIComponent(path)}`,
    )
  } catch {
    throw new Error('Page not found')
  }
}

export async function createPage({
  title,
  slug,
  parentId,
}: {
  title: string
  slug: string
  parentId: string | null
}) {
  if (parentId === '') parentId = null

  return await fetchWithAuth(`/api/pages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, slug, parentId }),
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
) {
  return await fetchWithAuth(`/api/pages/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, slug, content }),
  })
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
