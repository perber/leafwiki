import TreeView from "../features/tree/TreeView";
import { useSelectedPage } from "../stores/selectedPage";

export default function Sidebar() {

    const deselectPage = useSelectedPage(state => state.deselectPage)

    return (
        <aside className="w-64 border-r border-gray-200 bg-white p-4" onClick={(e) => {
            // Deselect the page when clicking on the tree view
            // but not when clicking on a node
            console.log("deselectPage")
            if (e.target === e.currentTarget) {
                deselectPage()
            }
        }}>
            <h2 className="text-xl font-bold mb-4">🌿 LeafWiki</h2>
            <TreeView />
        </aside>
    )
}