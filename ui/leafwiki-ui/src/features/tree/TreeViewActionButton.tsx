import { TooltipWrapper } from '@/components/TooltipWrapper'
import { ComponentPropsWithoutRef, forwardRef } from 'react'

type TreeViewActionButtonProps = {
  actionName: string
  icon: React.ReactNode
  tooltip: string
} & ComponentPropsWithoutRef<'button'>

export const TreeViewActionButton = forwardRef<
  HTMLButtonElement,
  TreeViewActionButtonProps
>(function TreeViewActionButton(
  { onClick, icon, actionName, tooltip, type = 'button', ...props },
  ref,
) {
  return (
    <div className="group mr-2 flex">
      <TooltipWrapper label={tooltip} side="top" align="start">
        <button
          {...props}
          ref={ref}
          type={type}
          onClick={(e) => {
            onClick?.(e)
            e.stopPropagation()
          }}
          className="btn-treeview"
          aria-label={tooltip}
          data-testid={`tree-view-action-button-${actionName}`}
        >
          {icon}
        </button>
      </TooltipWrapper>
    </div>
  )
})
