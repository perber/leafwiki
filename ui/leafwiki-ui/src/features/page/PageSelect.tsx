import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { PageNode } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { JSX, useMemo } from 'react'

export function PageSelect({
    pageID,
    onChange,
}: {
    pageID: string
    onChange: (id: string) => void
}) {
    const { tree } = useTreeStore()

    const selectOptions = useMemo(() => {
        if (!tree) return null
        const renderOptions = (node: PageNode, depth = 1): JSX.Element[] => {
            const indent = '—'.repeat(depth)
            const options = [
                <SelectItem key={node.id} value={node.id}>
                    {indent} {node.title}
                </SelectItem>,
            ]
            if (node.children?.length) {
                for (const child of node.children) {
                    options.push(...renderOptions(child, depth + 1))
                }
            }
            return options
        }
        return (
            <>
                <SelectItem key="root" value="root">
                    ⬆️ Top Level
                </SelectItem>
                {tree.children?.flatMap((child) => renderOptions(child))}
            </>
        )
    }, [tree])

    return (<Select value={pageID} onValueChange={onChange}>
        <SelectTrigger>
            <SelectValue placeholder="Select new parent..." />
        </SelectTrigger>
        <SelectContent>
            {selectOptions}
        </SelectContent>
    </Select>)
}
