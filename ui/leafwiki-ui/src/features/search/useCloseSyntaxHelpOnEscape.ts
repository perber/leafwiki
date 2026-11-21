import { createHotkeyDefinition } from '@/lib/shortcuts/shortcutCatalog'
import { useHotKeysStore } from '@/stores/hotkeys'
import { RefObject, useEffect } from 'react'

type UseCloseSyntaxHelpOnEscapeOptions = {
  // Open and on screen: the sidebar currently shows the search panel.
  enabled: boolean
  setOpen: (open: boolean) => void
  buttonRef: RefObject<HTMLButtonElement | null>
}

// Escape goes through the hotkey store rather than an onKeyDown: the
// HotKeyHandler stops Escape in the capture phase for the page's own hotkey
// (e.g. history.page.close). Registered only while enabled, so it sits on top
// of the page's Escape, and removed again on close or unmount. Takes only
// stable inputs, so a re-render never re-registers it: that would move it
// above an Escape registered later.
export function useCloseSyntaxHelpOnEscape({
  enabled,
  setOpen,
  buttonRef,
}: UseCloseSyntaxHelpOnEscapeOptions) {
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)

  useEffect(() => {
    if (!enabled) return

    const hotkey = createHotkeyDefinition(
      'search.syntaxHelp.close',
      () => {
        setOpen(false)
        // Focus inside the panel would otherwise drop with the hidden panel.
        buttonRef.current?.focus()
      },
      {
        // Escape in the editor belongs to CodeMirror (autocomplete, search
        // panel): when this returns false, HotKeyHandler leaves the event
        // alone. Trade-off: while the panel is open and focus is in the
        // editor, Escape neither closes the panel nor runs the editor's own
        // Escape hotkey, because the handler only checks the top entry.
        shouldHandle: () => !document.activeElement?.closest('.cm-editor'),
      },
    )
    registerHotkey(hotkey)

    return () => unregisterHotkey(hotkey)
  }, [enabled, setOpen, buttonRef, registerHotkey, unregisterHotkey])
}
