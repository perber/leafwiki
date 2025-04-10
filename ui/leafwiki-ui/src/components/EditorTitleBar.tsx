import { Input } from '@/components/ui/input'
import { Pencil } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

type Props = {
    title: string
    slug: string
    onChange: (newTitle: string) => void
}

export function EditorTitleBar({ title, slug, onChange }: Props) {
    const [editing, setEditing] = useState(false)
    const [value, setValue] = useState(title)
    const inputRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        if (!editing) {
            setValue(title)
        }
    }, [title, editing])

    useEffect(() => {
        if (editing) inputRef.current?.focus()
    }, [editing])

    const isDirty = value.trim() !== title.trim()

    const submit = () => {
        setEditing(false)
        if (isDirty) {
            onChange(value.trim())
        }
    }


    return (
        <div className="flex flex-col items-center">
            {editing ? (
                <Input
                    ref={inputRef}
                    className="h-8 w-64 text-base font-semibold text-center"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    onBlur={submit}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            e.preventDefault()
                            submit()
                        } else if (e.key === 'Escape') {
                            setEditing(false)
                            setValue(title)
                        }
                    }}
                />
            ) : (
                <button
                    onClick={() => setEditing(true)}
                    className="group flex items-center gap-1 text-base font-semibold text-gray-800 hover:underline"
                >
                    {title && (
                        <span>{title}</span>
                    )}
                    <Pencil size={16} className="text-gray-400 group-hover:text-gray-600" />
                    {isDirty && (
                        <span className="ml-2 text-xs text-yellow-600">(Bearbeitet)</span>
                    )}
                </button>
            )}

            <span className="mt-1 rounded bg-gray-200 px-2 py-0.5 text-xs font-mono text-gray-700">
                {slug}
            </span>
        </div>
    )
}
