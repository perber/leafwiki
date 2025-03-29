const dummyTree = [
    { id: '1', title: 'Home', slug: 'home' },
    { id: '2', title: 'Docs', slug: 'docs', children: [
      { id: '3', title: 'API', slug: 'api' },
      { id: '4', title: 'Auth', slug: 'auth' },
    ]},
    { id: '5', title: 'About', slug: 'about' }
  ]
  
  export default function Sidebar() {
    return (
      <aside className="w-64 border-r border-gray-200 bg-white p-4">
        <h2 className="text-xl font-bold mb-4">🌿 LeafWiki</h2>
        <nav className="space-y-2">
          {dummyTree.map(node => (
            <TreeNode key={node.id} node={node} level={0} />
          ))}
        </nav>
      </aside>
    )
  }
  
  function TreeNode({ node, level }: { node: any, level: number }) {
    return (
      <div className={`pl-${level * 4} ml-2`}>
        <div className="text-sm text-gray-800 cursor-pointer hover:underline">
          {node.title}
        </div>
        {node.children?.map((child: any) => (
          <TreeNode key={child.id} node={child} level={level + 1} />
        ))}
      </div>
    )
  }