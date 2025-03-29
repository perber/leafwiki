import { API_BASE_URL } from "./config"

export type PageNode = {
    id: string
    title: string
    slug: string
    children: PageNode[]
  }
  
  export async function fetchTree(): Promise<PageNode> {
    const res = await fetch(`${API_BASE_URL}/api/tree`)
    if (!res.ok) throw new Error("Failed to load tree")
    return await res.json()
  }
  