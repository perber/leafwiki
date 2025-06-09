import { TooltipWrapper } from './TooltipWrapper'

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
        <button type="button" onClick={() => onClick()} className="btn-treeview">
          {icon}
        </button>
      </TooltipWrapper>
    </div>
  )
}
