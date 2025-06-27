import { TooltipWrapper } from '@/components/TooltipWrapper'

type TreeViewActionButtonProps = {
  onClick: () => void
  icon: React.ReactNode
  tooltip: string
}

export function TreeViewActionButton({
  onClick,
  icon,
  tooltip,
}: TreeViewActionButtonProps) {
  return (
    <div className="group mr-2 flex">
      <TooltipWrapper label={tooltip} side="top" align="start">
        <button
          type="button"
          onClick={() => onClick()}
          className="btn-treeview"
          aria-label={tooltip}
        >
          {icon}
        </button>
      </TooltipWrapper>
    </div>
  )
}
