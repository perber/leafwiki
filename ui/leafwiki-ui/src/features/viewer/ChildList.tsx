import { Button } from "@/components/ui/button"
import { NODE_KIND_SECTION, Page } from "@/lib/api/pages"
import { DIALOG_ADD_PAGE } from "@/lib/registries"
import { useDialogsStore } from "@/stores/dialogs"
import { useTreeStore } from "@/stores/tree"
import { Link } from "react-router-dom"

type ChildListProps = {
    page: Page
}

export default function ChildList({ page }: ChildListProps) {
    const getPageById = useTreeStore((s) => s.getPageById)
    const node = getPageById(page.id)
    const openDialog = useDialogsStore((s) => s.openDialog)
    const tree = useTreeStore((s) => s.tree)

    if (!tree) {
        return null
    }
    
    if (page.kind !== NODE_KIND_SECTION) {
        return null
    }

    // If the page has content, do not show the child list
    if (page.content && page.content.trim().length > 0 ) {
        return null
    }

    if (!node ) {
        return null
    }

    const hasChildren = node.children && node.children.length > 0

    return (
        <>
        {hasChildren && (
            <div className="section-content__subpages">
                <h2>Pages and Sections in {page.title}</h2>
                <ul>
                    {node.children?.map((n) => {
                        if (!n) return null
                        return (
                            <li key={n.id}>
                                <Link to={`/${n.path}`}>{n.title}</Link> {n.kind === NODE_KIND_SECTION && ' (Section)'} 
                                {/* Last edited info */}
                                {}

                            </li>
                        )
                    })}
                </ul>
            </div>
        )}
        {/* No children - Add Button and allow users to create a new page */}
        {!hasChildren && (
            <div className="section-content__no-children">
                <p>No child pages found.</p>
                <Button onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: page.id })} variant="outline">
                    Add Child Page
                </Button>
            </div>
        )}
        </>
    )
}