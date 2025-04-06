import TreeView from '../features/tree/TreeView'

export default function Sidebar() {
  return (
    <aside className="h-screen w-96 border-r border-gray-200 bg-white p-4 shadow-md">
      <h2 className="mb-4 text-xl font-bold">🌿 LeafWiki</h2>
      <TreeView />
    </aside>
  )
}
