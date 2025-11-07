import { useTreeStore } from '@/stores/tree'
import { Navigate } from 'react-router-dom'

export default function RootRedirect() {
  const { tree } = useTreeStore()

  if (!tree || !tree.children || tree.children.length === 0) return null

  const first = tree.children[0]
  return <Navigate to={`/${first.path}`} replace />
}
