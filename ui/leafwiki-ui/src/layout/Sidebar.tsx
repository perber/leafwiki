import TreeView from '../features/tree/TreeView'

export default function Sidebar() {
  return (
    <aside className="w-64 border-r border-gray-200 bg-white p-4">
      <h2 className="mb-4 text-xl font-bold">🌿 LeafWiki</h2>
      <TreeView />
    </aside>
  )
}
