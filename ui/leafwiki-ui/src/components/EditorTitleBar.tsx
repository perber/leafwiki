import { useIsMobile } from '@/lib/useIsMobile'
import { Pencil } from 'lucide-react'
import { TooltipWrapper } from './TooltipWrapper'

type Props = {
  title: string
  slug: string
  onEditClicked: () => void
  isDirty?: boolean
}

export function EditorTitleBar({ title, slug, onEditClicked, isDirty }: Props) {
  const isMobile = useIsMobile()

  return (
    <div className="flex flex-1 flex-col items-center">
      <button
        onClick={() => onEditClicked()}
        className="group relative flex items-center gap-1 text-base font-semibold text-gray-800 hover:underline"
      >
        <TooltipWrapper label={title} side="top" align="start">
          {title && (
            <span className="inline-block max-w-[15vw] truncate sm:max-w-[40vw]">
              {title}
            </span>
          )}
          <Pencil
            size={16}
            className="absolute -right-6 top-1/2 -translate-y-1/2 text-gray-400 transition-transform duration-200 ease-in-out group-hover:text-gray-600"
          />
          {isDirty && !isMobile && (
            <span className="ml-2 text-xs text-yellow-600">(Changes)</span>
          )}

          {isDirty && isMobile && (
            <span className="ml-2 text-xs text-yellow-600">*</span>
          )}
        </TooltipWrapper>
      </button>
      <span className="mt-1 inline-block max-w-[15vw] truncate rounded bg-gray-200 px-2 py-0.5 font-mono text-xs text-gray-700 sm:max-w-[40vw]">
        {slug}
      </span>
    </div>
  )
}
