// dialogRegistry
// A registry for dialog components to be used in the application.

import { JSX } from "react/jsx-runtime"

export interface Dialog {
  type: string
  render: (props: unknown) => JSX.Element
}

export class DialogRegistry {
  private dialogs: Map<string, Dialog> = new Map()

  register(dialog: Dialog) {
    if (this.dialogs.has(dialog.type)) {
      throw new Error(`Dialog with type ${dialog.type} is already registered.`)
    }
    this.dialogs.set(dialog.type, dialog)
  }

  getDialog(type: string): Dialog | undefined {
    return this.dialogs.get(type)
  }

  getAllDialogs(): Dialog[] {
    return Array.from(this.dialogs.values())
  }
}