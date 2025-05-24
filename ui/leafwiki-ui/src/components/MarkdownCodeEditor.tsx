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
import { useEffect, useRef } from 'react'

type MarkdownCodeEditorProps = {
  initialValue: string
  onChange: (value: string) => void
  onCursorLineChange?: (line: number) => void
  insert?: string | null
}

export default function MarkdownCodeEditor({
  initialValue,
  insert,
  onChange,
  onCursorLineChange,
}: MarkdownCodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const onChangeRef = useRef(onChange)
  const valueRef = useRef(initialValue)

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

    const state = EditorState.create({
      doc: initialValue,
      extensions: [
        oneDark,
        markdown(),
        history(),
        keymap.of([indentWithTab, ...defaultKeymap, ...historyKeymap]),
        EditorView.lineWrapping,
        updateListener,
        EditorView.theme({
          '&': { height: '100%' },
          '.cm-editor': { height: '100%' },
          '.cm-scroller': { height: '100%' },
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

    return () => {
      view.destroy()
      viewRef.current = null
    }
  }, [initialValue, onCursorLineChange])

  // Sync external value only when it comes from outside (not typed in editor)
  useEffect(() => {
    const view = viewRef.current
    if (!view || !insert) return

    const { from } = view.state.selection.main
    view.dispatch({
      changes: { from, insert },
      selection: { anchor: from + insert.length },
    })

    const newDoc = view.state.doc.toString()
    valueRef.current = newDoc
    onChangeRef.current(newDoc)
  }, [insert])

  return (
    <div ref={editorRef} className="h-full w-1/2 rounded border shadow-sm" />
  )
}
