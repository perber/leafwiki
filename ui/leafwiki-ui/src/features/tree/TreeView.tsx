import { useTreeStore } from '@/stores/tree'
import React, { useEffect } from 'react'
import { TreeAddInline } from './TreeAddInline'
import { TreeNode } from './TreeNode'
export default function TreeView() {
  const { tree, loading, error, reloadTree } = useTreeStore()

  useEffect(() => {
    reloadTree()
  }, [reloadTree])

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  return (
    <div className="space-y-1">
      {tree.children.map((node) => (
        <React.Fragment key={node.id}>
          <TreeNode node={node} />
        </React.Fragment>
      ))}
      <div className="ml-2">
        <TreeAddInline parentId={''} minimal />
      </div>
    </div>
  )
}
