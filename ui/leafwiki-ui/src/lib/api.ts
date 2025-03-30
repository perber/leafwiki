import { API_BASE_URL } from './config'

export type PageNode = {
  id: string
  title: string
  slug: string
  path: string
  children: PageNode[]
}

export async function fetchTree(): Promise<PageNode> {
  const res = await fetch(`${API_BASE_URL}/api/tree`)
  if (!res.ok) throw new Error('Failed to load tree')
  return await res.json()
}

export async function suggestSlug(
  parentId: string,
  title: string,
): Promise<string> {
  const res = await fetch(
    `${API_BASE_URL}/api/pages/slug-suggestion?parentID=${parentId}&title=${encodeURIComponent(title)}`,
  )
  const data = await res.json()
  return data.slug
}

export async function getPageByPath(path: string) {
  const res = await fetch(
    `${API_BASE_URL}/api/pages/by-path?path=${encodeURIComponent(path)}`,
  )
  if (!res.ok) throw new Error('Page not found')
  return res.json()
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
  const res = await fetch(`${API_BASE_URL}/api/pages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, slug, parentId }),
  })
  if (!res.ok) throw new Error('Seite konnte nicht erstellt werden')
  return await res.json()
}

export async function updatePage(id: string, title: string, slug: string, content: string) {
  const res = await fetch(`${API_BASE_URL}/api/pages/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, slug, content }),
  })
  if (!res.ok) throw new Error("Update failed")
}

export async function deletePage(id: string) {
  const res = await fetch(`${API_BASE_URL}/api/pages/${id}`, {
    method: "DELETE",
  })
  if (!res.ok) throw new Error("Delete failed")
}

export async function movePage(id: string, parentId: string | null) {

  console.log("parentID ", parentId)

  if (parentId === '' || parentId == "root") parentId = null

  const res = await fetch(`${API_BASE_URL}/api/pages/${id}/move`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ parentId }),
  })
  if (!res.ok) throw new Error("Move failed")
}