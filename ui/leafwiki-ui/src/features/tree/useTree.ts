import { useEffect, useState } from "react"
import { fetchTree, PageNode } from "../../lib/api"

export function useTree() {
  const [tree, setTree] = useState<PageNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchTree()
      .then(setTree)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  return { tree, loading, error }
}