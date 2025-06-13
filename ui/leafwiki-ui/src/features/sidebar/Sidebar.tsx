import TreeView from '../tree/TreeView'

export default function Sidebar() {
  return (
    <aside key={'sidebar'}>
      <h2 className="mb-4 text-xl font-bold">🌿 LeafWiki</h2>
      <TreeView />
    </aside>
  )
}
