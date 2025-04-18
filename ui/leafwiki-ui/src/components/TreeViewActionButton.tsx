import { TooltipWrapper } from "./TooltipWrapper"

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
        <button onClick={() => onClick()}>{icon}</button>
      </TooltipWrapper>
    </div>
  )
}
