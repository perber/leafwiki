import { AccordionItem } from '@/components/ui/accordion'
import { cn } from '@/lib/utils'
import * as AccordionPrimitive from '@radix-ui/react-accordion'
import { ChevronDown } from 'lucide-react'
import { ReactNode } from 'react'

type SidebarAccordionSectionProps = {
  value: string
  title: ReactNode
  icon?: ReactNode
  count?: number
  actions?: ReactNode
  collapseToggleLabel: string
  children: ReactNode
  className?: string
}

export function SidebarAccordionSection({
  value,
  title,
  icon,
  count,
  actions,
  collapseToggleLabel,
  children,
  className,
}: SidebarAccordionSectionProps) {
  return (
    <AccordionItem
      value={value}
      className={cn('sidebar-accordion-section', className)}
    >
      <AccordionPrimitive.Header className="sidebar-accordion-section__header">
        <div className="sidebar-accordion-section__title">
          {icon}
          <span>{title}</span>
          {typeof count === 'number' && (
            <span className="sidebar-accordion-section__count">{count}</span>
          )}
        </div>
        <div className="sidebar-accordion-section__actions">
          {actions}
          <AccordionPrimitive.Trigger asChild>
            <button
              type="button"
              className="sidebar-accordion-section__trigger"
              aria-label={collapseToggleLabel}
            >
              <ChevronDown size={14} />
            </button>
          </AccordionPrimitive.Trigger>
        </div>
      </AccordionPrimitive.Header>
      <AccordionPrimitive.Content className="sidebar-accordion-section__panel data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down overflow-hidden">
        <div className="sidebar-accordion-section__content">{children}</div>
      </AccordionPrimitive.Content>
    </AccordionItem>
  )
}
