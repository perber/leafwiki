import { Pencil } from 'lucide-react'

type Props = {
  title: string
  slug: string
  onEditClicked: () => void
  isDirty?: boolean
}

export function EditorTitleBar({ title, slug, onEditClicked, isDirty }: Props) {

  return (
    <div className="flex flex-col items-center">
      <button
        onClick={() => onEditClicked()}
        className="group flex items-center gap-1 text-base font-semibold text-gray-800 hover:underline relative"
      >
        {title && <span>{title}</span>}
        <Pencil
          size={16}
          className="text-gray-400 group-hover:text-gray-600 absolute -right-6 top-1/2 -translate-y-1/2 transition-transform duration-200 ease-in-out"
        />
        {isDirty && (
          <span className="ml-2 text-xs text-yellow-600">(Bearbeitet)</span>
        )}
      </button>

      <span className="mt-1 rounded bg-gray-200 px-2 py-0.5 font-mono text-xs text-gray-700">
        {slug}
      </span>
    </div>
  )
}
