import { Accordion } from '@/components/ui/accordion'
import { fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { SidebarAccordionSection } from './SidebarAccordionSection'

function ControlledAccordion({
  defaultValue,
  onValueChangeSpy,
}: {
  defaultValue: string[]
  onValueChangeSpy?: (value: string[]) => void
}) {
  const [value, setValue] = useState(defaultValue)
  return (
    <Accordion
      type="multiple"
      value={value}
      onValueChange={(next) => {
        setValue(next)
        onValueChangeSpy?.(next)
      }}
    >
      <SidebarAccordionSection
        value="pinned"
        title="Pinned"
        collapseToggleLabel="Toggle pinned pages section"
        actions={<button>Add page</button>}
      >
        <div data-testid="pinned-content">pinned content</div>
      </SidebarAccordionSection>
    </Accordion>
  )
}

describe('SidebarAccordionSection', () => {
  it('renders the title and children', () => {
    render(<ControlledAccordion defaultValue={['pinned']} />)
    expect(screen.getByText('Pinned')).toBeInTheDocument()
    expect(screen.getByTestId('pinned-content')).toBeInTheDocument()
  })

  it('renders actions in the header, always visible', () => {
    render(<ControlledAccordion defaultValue={[]} />)
    expect(screen.getByText('Add page')).toBeInTheDocument()
  })

  it('collapses the section when the chevron toggle is clicked', () => {
    const onValueChangeSpy = vi.fn()
    render(
      <ControlledAccordion
        defaultValue={['pinned']}
        onValueChangeSpy={onValueChangeSpy}
      />,
    )

    fireEvent.click(
      screen.getByRole('button', { name: 'Toggle pinned pages section' }),
    )

    expect(onValueChangeSpy).toHaveBeenCalledWith([])
  })

  it('expands the section when the chevron toggle is clicked while closed', () => {
    const onValueChangeSpy = vi.fn()
    render(
      <ControlledAccordion
        defaultValue={[]}
        onValueChangeSpy={onValueChangeSpy}
      />,
    )

    fireEvent.click(
      screen.getByRole('button', { name: 'Toggle pinned pages section' }),
    )

    expect(onValueChangeSpy).toHaveBeenCalledWith(['pinned'])
  })

  it('clicking an action button does not toggle the section', () => {
    const onValueChangeSpy = vi.fn()
    render(
      <ControlledAccordion
        defaultValue={['pinned']}
        onValueChangeSpy={onValueChangeSpy}
      />,
    )

    fireEvent.click(screen.getByText('Add page'))

    expect(onValueChangeSpy).not.toHaveBeenCalled()
  })
})
