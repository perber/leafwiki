import {
  autocompletion,
  closeCompletion,
  completionStatus,
} from '@codemirror/autocomplete'
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from '@codemirror/commands'
import { markdown } from '@codemirror/lang-markdown'
import { openSearchPanel, search, searchKeymap } from '@codemirror/search'
import { Compartment, EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { EditorView, keymap } from '@codemirror/view'
import { githubLight } from '@fsegurai/codemirror-theme-github-light'
import { useEffect, useRef, useState } from 'react'
import { useDesignModeStore } from '../designtoggle/designmode'
import { insertHeadingAtStart, insertWrappedText } from './editorCommands'
import type { InternalLinkCompletion } from './internalLinkCompletion'
import { internalLinkCompletionSource } from './internalLinkCompletion'

// Extensions toggled via lineWrapCompartment
const noWrapExtensions = EditorView.theme({
  '.cm-content': { whiteSpace: 'pre', width: 'max-content' },
  '.cm-line': { whiteSpace: 'pre' },
})

// font-size is 13px; 1.5*13 + 3 + 3 = 25.5px — matches "Enter" spacing
const wrapExtensions = [
  EditorView.lineWrapping,
  EditorView.theme({
    '.cm-content': { lineHeight: 'calc(1.5em + 6px)' },
    '.cm-line': {
      lineHeight: 'calc(1.5em + 6px)',
      paddingTop: '0',
      paddingBottom: '0',
    },
  }),
]

type MarkdownCodeEditorProps = {
  initialValue: string
  onChange: (value: string) => void
  onCursorLineChange?: (line: number) => void
  editorViewRef: React.RefObject<EditorView | null>
  lineWrap?: boolean
}

// CodeMirror uses 80 for the built-in detail slot, so render the path just before it.
const COMPLETION_PATH_POSITION_BEFORE_DETAIL = 79

function openReplacePanel(view: EditorView) {
  openSearchPanel(view)

  requestAnimationFrame(() => {
    if (!view.dom.isConnected) return
    const replaceField = view.dom.querySelector(
      '.cm-search input[name="replace"]',
    ) as HTMLInputElement | null

    replaceField?.focus()
    replaceField?.select()
  })

  return true
}

export default function MarkdownCodeEditor({
  initialValue,
  editorViewRef,
  onChange,
  onCursorLineChange,
  lineWrap = true,
}: MarkdownCodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const onChangeRef = useRef(onChange)
  const valueRef = useRef(initialValue)

  const designMode = useDesignModeStore((state) => state.mode)
  const [themeCompartment] = useState(() => new Compartment())
  const [lineWrapCompartment] = useState(() => new Compartment())

  // Always use the latest onChange function
  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  // Initial editor setup (only once)
  useEffect(() => {
    if (!editorRef.current) return

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const newValue = update.state.doc.toString()
        valueRef.current = newValue // Update internal tracker
        onChangeRef.current(newValue)
      }

      if (update.selectionSet && onCursorLineChange) {
        const pos = update.state.selection.main.head
        const line = update.state.doc.lineAt(pos).number
        onCursorLineChange(line)
      }
    })

    const customShortcuts = [
      {
        key: 'Mod-h',
        run: openReplacePanel,
        preventDefault: true,
      },
      {
        key: 'Mod-b',
        run: (view: EditorView) => {
          insertWrappedText(view, '**', '**')
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Mod-i',
        run: (view: EditorView) => {
          insertWrappedText(view, '_', '_')
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Mod-`',
        run: (view: EditorView) => {
          insertWrappedText(view, '`', '`')
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Mod-Alt-1',
        run: (view: EditorView) => {
          insertHeadingAtStart(view, 1)
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Mod-Alt-2',
        run: (view: EditorView) => {
          insertHeadingAtStart(view, 2)
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Mod-Alt-3',
        run: (view: EditorView) => {
          insertHeadingAtStart(view, 3)
          return true
        },
        preventDefault: true,
      },
      {
        key: 'Escape',
        run: (view: EditorView) => {
          if (completionStatus(view.state) === null) {
            return false
          }

          return closeCompletion(view)
        },
        stopPropagation: true,
      },
    ]

    const state = EditorState.create({
      doc: initialValue,
      extensions: [
        themeCompartment.of(designMode === 'light' ? githubLight : oneDark),
        lineWrapCompartment.of(lineWrap ? wrapExtensions : noWrapExtensions),
        markdown(),
        search({
          top: true,
        }),
        autocompletion({
          override: [internalLinkCompletionSource],
          icons: false,
          optionClass: () => 'cm-internal-link-option',
          addToOptions: [
            {
              render: (completion) => {
                const option = completion as InternalLinkCompletion
                const path = document.createElement('div')
                path.className = 'cm-internal-link-option__path'
                path.textContent = `/${option.path}`
                return path
              },
              position: COMPLETION_PATH_POSITION_BEFORE_DETAIL,
            },
          ],
        }),
        history(),
        keymap.of([
          ...customShortcuts,
          ...searchKeymap,
          indentWithTab,
          ...historyKeymap,
          ...defaultKeymap,
        ]),
        // Prevent Chrome from applying its own italic/bold formatting via beforeinput
        // when Ctrl+I / Ctrl+B is pressed. CodeMirror's keymap already handles these.
        EditorView.domEventHandlers({
          beforeinput(event) {
            if (
              event.inputType === 'formatItalic' ||
              event.inputType === 'formatBold' ||
              event.inputType === 'formatCode'
            ) {
              event.preventDefault()
            }
          },
        }),
        updateListener,
        EditorView.theme({
          '&': {
            height: '100%',
            backgroundColor: 'hsl(var(--surface-alt)) !important',
            fontSize: '13px !important', // gleiche Größe wie githubLight
            fontFamily: 'monospace !important',
            color: 'hsl(var(--interface-text)) !important',
          },
          '.cm-editor': { height: '100%' },
          '.cm-scroller': {
            height: '100%',
            overflowX: 'auto',
            overflowY: 'auto',
          },
          '.cm-content': {
            lineHeight: '1.5',
            minWidth: '100%',
          },
          '.cm-line': {
            lineHeight: '1.5',
            paddingTop: '3px',
            paddingBottom: '3px',
            paddingLeft: '15px',
          },
          '.cm-gutters': {
            lineHeight: '1.5',
          },
          '.cm-panels': {
            backgroundColor: 'hsl(var(--surface))',
            color: 'hsl(var(--interface-text))',
          },
          '.cm-panel.cm-search': {
            borderBottom: '1px solid hsl(var(--surface-border))',
            padding: '10px 12px 8px',
            gap: '4px',
          },
          '.cm-panel.cm-search [name="close"]': {
            color: 'hsl(var(--muted-foreground))',
            cursor: 'pointer',
          },
          '.cm-panel.cm-search label': {
            display: 'inline-flex',
            alignItems: 'center',
            gap: '6px',
            fontSize: '12px',
          },
          '.cm-panel.cm-search input.cm-textfield': {
            border: '1px solid hsl(var(--surface-border))',
            borderRadius: '6px',
            backgroundColor: 'hsl(var(--surface-alt))',
            color: 'hsl(var(--interface-text))',
            padding: '6px 8px',
            minWidth: '140px',
          },
          '.cm-panel.cm-search input.cm-textfield:focus': {
            outline: '2px solid hsl(var(--ring))',
            outlineOffset: '1px',
          },
          '.cm-panel.cm-search button.cm-button': {
            border: '1px solid hsl(var(--surface-border))',
            borderRadius: '6px',
            backgroundColor: 'hsl(var(--surface-alt))',
            color: 'hsl(var(--interface-text))',
            padding: '6px 10px',
            cursor: 'pointer',
          },
          '.cm-panel.cm-search button.cm-button:hover': {
            backgroundColor: 'hsl(var(--accent))',
          },
          '.cm-panel.cm-search button.cm-button:disabled': {
            cursor: 'not-allowed',
            opacity: '0.6',
          },
          '.cm-searchMatch': {
            backgroundColor: 'hsl(var(--warning) / 0.22)',
          },
          '.cm-searchMatch.cm-searchMatch-selected': {
            backgroundColor: 'hsl(var(--primary) / 0.28)',
          },
          '&.cm-focused': {
            outline: 'none',
          },
        }),
      ],
    })

    const view = new EditorView({
      state,
      parent: editorRef.current,
    })

    viewRef.current = view
    editorViewRef.current = view

    requestAnimationFrame(() => {
      view.focus()
    })

    return () => {
      view.destroy()
      viewRef.current = null
      editorViewRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue, onCursorLineChange, editorViewRef, themeCompartment])

  useEffect(() => {
    const view = viewRef.current
    if (!view) return
    view.dispatch({
      effects: themeCompartment.reconfigure(
        designMode === 'light' ? githubLight : oneDark,
      ),
    })
  }, [designMode, themeCompartment])

  useEffect(() => {
    const view = viewRef.current
    if (!view) return
    view.dispatch({
      effects: lineWrapCompartment.reconfigure(
        lineWrap ? wrapExtensions : noWrapExtensions,
      ),
    })
  }, [lineWrap, lineWrapCompartment])

  return <div ref={editorRef} className="markdown-code-editor" />
}
