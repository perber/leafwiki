import { TooltipWrapper } from '@/components/TooltipWrapper'

type TreeViewActionButtonProps = {
  onClick: () => void
  actionName: string
  icon: React.ReactNode
  tooltip: string
}

export function TreeViewActionButton({
  onClick,
  icon,
  actionName,
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
          data-testid={`tree-view-action-button-${actionName}`}
        >
          {icon}
        </button>
      </TooltipWrapper>
    </div>
  )
}
