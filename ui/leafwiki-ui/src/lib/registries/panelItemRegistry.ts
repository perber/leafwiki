// PanelItemRegistry
// This file is used to register panel items for the sidebar or other panels.

import { JSX } from 'react'

export interface PanelItem {
  id: string
  label: string
  icon: () => JSX.Element
  render: (props: unknown) => JSX.Element
}

export class PanelItemRegistry {
  private items: Map<string, PanelItem> = new Map()

  register(item: PanelItem) {
    if (this.items.has(item.id)) {
      throw new Error(`Panel item with id ${item.id} is already registered.`)
    }
    this.items.set(item.id, item)
  }

  getItem(id: string): PanelItem | undefined {
    return this.items.get(id)
  }

  getAllItems(): PanelItem[] {
    return Array.from(this.items.values())
  }
}
