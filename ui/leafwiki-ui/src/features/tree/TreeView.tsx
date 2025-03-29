import { TreeNode } from "./TreeNode"
import { useTree } from "./useTree"
export default function TreeView() {
  const { tree, loading, error } = useTree()

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree) return <p className="text-sm text-red-500">Error: {error}</p>

  return (
    <div className="space-y-1">
      {tree.children.map(node => (
        <TreeNode key={node.id} node={node} />
      ))}
    </div>
  )
}
