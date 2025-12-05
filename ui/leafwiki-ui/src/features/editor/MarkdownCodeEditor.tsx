import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from '@codemirror/commands'
import { markdown } from '@codemirror/lang-markdown'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { EditorView, keymap } from '@codemirror/view'
import { githubLight } from '@fsegurai/codemirror-theme-github-light'
import { useEffect, useRef } from 'react'
import { useDesignModeStore } from '../designtoggle/designmode'
import { insertHeadingAtStart, insertWrappedText } from './editorCommands'

type MarkdownCodeEditorProps = {
  initialValue: string
  onChange: (value: string) => void
  onCursorLineChange?: (line: number) => void
  editorViewRef: React.RefObject<EditorView | null>
}

export default function MarkdownCodeEditor({
  initialValue,
  editorViewRef,
  onChange,
  onCursorLineChange,
}: MarkdownCodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const onChangeRef = useRef(onChange)
  const valueRef = useRef(initialValue)

  const designMode = useDesignModeStore((state) => state.mode)

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
        key: 'Mod-b',
        run: () => {
          const view = editorViewRef.current
          if (!view) return false
          insertWrappedText(view, '**', '**')
          return true
        },
      },
      {
        key: 'Mod-i',
        run: () => {
          const view = editorViewRef.current
          if (!view) return false
          insertWrappedText(view, '_', '_')
          return true
        },
      },
      {
        key: 'Mod-Alt-1',
        run: () => {
          const view = editorViewRef.current
          if (!view) return false
          insertHeadingAtStart(view, 1)
          return true
        },
      },
      {
        key: 'Mod-Alt-2',
        run: () => {
          const view = editorViewRef.current
          if (!view) return false
          insertHeadingAtStart(view, 2)
          return true
        },
      },
      {
        key: 'Mod-Alt-3',
        run: () => {
          const view = editorViewRef.current
          if (!view) return false
          insertHeadingAtStart(view, 3)
          return true
        },
      },
    ]

    const state = EditorState.create({
      doc: initialValue,
      extensions: [
        designMode === 'light' ? githubLight : oneDark,
        markdown(),
        history(),
        keymap.of([
          ...customShortcuts,
          indentWithTab,
          ...historyKeymap,
          ...defaultKeymap,
        ]),
        EditorView.lineWrapping,
        updateListener,
        EditorView.theme({
          '&': {
            height: '100%',
            backgroundColor: 'hsl(var(--surface-alt)) !important',
            fontSize: '13px !important', // gleiche Größe wie githubLight
            fontFamily: 'monospace !important',
            color: 'hsl(var(--surface-text)) !important',
          },
          '.cm-editor': { height: '100%' },
          '.cm-scroller': { height: '100%' },
          '.cm-content': {
            lineHeight: '1.5',
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
    }
  }, [initialValue, onCursorLineChange, editorViewRef, designMode])

  return <div ref={editorRef} className="markdown-code-editor" />
}
