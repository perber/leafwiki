import { API_BASE_URL } from "./config"

export type PageNode = {
    id: string
    title: string
    slug: string
    path: string
    children: PageNode[]
  }
  
  export async function fetchTree(): Promise<PageNode> {
    const res = await fetch(`${API_BASE_URL}/api/tree`)
    if (!res.ok) throw new Error("Failed to load tree")
    return await res.json()
  }
  
  export async function suggestSlug(parentId: string, title: string): Promise<string> {
    const res = await fetch(`${API_BASE_URL}/api/pages/slug-suggestion?parentID=${parentId}&title=${encodeURIComponent(title)}`)
    const data = await res.json()
    return data.slug
  }
  
  export async function createPage({ title, slug, parentId }: { title: string, slug: string, parentId: string | null }) {
    if (parentId === "") parentId = null
    const res = await fetch(`${API_BASE_URL}/api/pages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title, slug, parentId }),
    })
    if (!res.ok) throw new Error("Seite konnte nicht erstellt werden")
    return await res.json()
  }
