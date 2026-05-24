import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DIALOG_ASSET_MANAGER, DIALOG_LINK_INSERT } from '@/lib/registries'
import { useIsMobile } from '@/lib/useIsMobile'
import { useDialogsStore } from '@/stores/dialogs'
import {
  Bold,
  Code,
  Code2,
  Eye,
  EyeOff,
  Image,
  Italic,
  Link,
  Redo,
  Strikethrough,
  Table,
  Undo,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { MarkdownEditorRef } from './MarkdownEditor'

type Props = {
  editorRef: React.RefObject<MarkdownEditorRef>
  onAssetVersionChange?: (version: number) => void
  pageId: string
  previewVisible: boolean
  onTogglePreview: () => void
}

export default function MarkdownToolbar({
  editorRef,
  onAssetVersionChange,
  pageId,
  previewVisible,
  onTogglePreview,
}: Props) {
  const openDialog = useDialogsStore((state) => state.openDialog)
  const [canUndo, setCanUndo] = useState(false)
  const [canRedo, setCanRedo] = useState(false)
  const isRenamingRef = useRef(false)
  const isMobile = useIsMobile()

  useEffect(() => {
    const check = () => {
      const editor = editorRef.current
      if (!editor) return
      setCanUndo(editor.canUndo())
      setCanRedo(editor.canRedo())
    }

    const interval = setInterval(check, 300)
    return () => clearInterval(interval)
  }, [editorRef])

  // Update asset version when modal opens
  // This is to ensure that the preview updates when assets are changed
  // Otherwise the preview might show stale assets (e.g., images) - Caching Issue
  const assetChangedHandler = useCallback(() => {
    if (onAssetVersionChange) {
      onAssetVersionChange(Date.now() || 0)
    }
  }, [onAssetVersionChange])

  const [tablePickerOpen, setTablePickerOpen] = useState(false)
  const [hovered, setHovered] = useState<{ col: number; row: number } | null>(
    null,
  )

  const buildTableMarkdown = (cols: number, rows: number): string => {
    const header =
      '| ' +
      Array.from({ length: cols }, (_, i) => `Header ${i + 1}`).join(' | ') +
      ' |'
    const divider = '|' + Array(cols).fill('----------').join('|') + '|'
    const row = '| ' + Array(cols).fill('Cell').join(' | ') + ' |'
    return [header, divider, ...Array(rows).fill(row)].join('\n')
  }

  return (
    <>
      <div className="markdown-toolbar">
        <TooltipWrapper label="Bold (Ctrl+B)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('**')}
            className="markdown-toolbar__button"
            data-testid="format-bold-button"
          >
            <Bold className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Italic (Ctrl+I)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('_')}
            className="markdown-toolbar__button"
            data-testid="format-italic-button"
          >
            <Italic className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Strikethrough" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('~~')}
            className="markdown-toolbar__button"
          >
            <Strikethrough className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Link (Ctrl+K)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button"
            onClick={() => {
              const view = editorRef.current?.editorViewRef.current
              const selectedText = view
                ? view.state.doc.sliceString(
                    view.state.selection.main.from,
                    view.state.selection.main.to,
                  )
                : ''
              openDialog(DIALOG_LINK_INSERT, { editorRef, selectedText })
            }}
          >
            <Link className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <div className="markdown-toolbar__separator" />
        <TooltipWrapper
          label="Heading 1 (Ctrl+Alt+1)"
          side="top"
          align="center"
        >
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button markdown-toolbar__button--desktop-only"
            onClick={() => editorRef.current?.insertHeading(1)}
          >
            H1
          </Button>
        </TooltipWrapper>
        <TooltipWrapper
          label="Heading 2 (Ctrl+Alt+2)"
          side="top"
          align="center"
        >
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button markdown-toolbar__button--desktop-only"
            onClick={() => editorRef.current?.insertHeading(2)}
          >
            H2
          </Button>
        </TooltipWrapper>
        <TooltipWrapper
          label="Heading 3 (Ctrl+Alt+3)"
          side="top"
          align="center"
        >
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button markdown-toolbar__button--desktop-only"
            onClick={() => editorRef.current?.insertHeading(3)}
          >
            H3
          </Button>
        </TooltipWrapper>
        <div className="markdown-toolbar__separator markdown-toolbar__separator--desktop-only" />
        <DropdownMenu
          open={tablePickerOpen}
          onOpenChange={(open) => {
            setTablePickerOpen(open)
            if (!open) {
              setHovered(null)
            }
          }}
        >
          <TooltipWrapper label="Table" side="top" align="center">
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
              >
                <Table className="markdown-toolbar__icon" />
              </Button>
            </DropdownMenuTrigger>
          </TooltipWrapper>
          <DropdownMenuContent
            className="p-2"
            onCloseAutoFocus={(e) => e.preventDefault()}
          >
            <div className="text-muted-foreground mb-1 text-center text-xs">
              {hovered ? `${hovered.col} × ${hovered.row}` : 'Select size'}
            </div>
            <div
              className="grid gap-0.5"
              style={{ gridTemplateColumns: 'repeat(6, 1fr)' }}
            >
              {Array.from({ length: 6 }, (_, rowIdx) =>
                Array.from({ length: 6 }, (_, colIdx) => {
                  const col = colIdx + 1
                  const row = rowIdx + 1
                  const highlighted =
                    hovered && col <= hovered.col && row <= hovered.row
                  return (
                    <button
                      key={`${col}-${row}`}
                      type="button"
                      aria-label={`Insert table ${col} x ${row}`}
                      className={`h-5 w-5 rounded-sm border transition-colors ${highlighted ? 'bg-primary border-primary' : 'border-border hover:bg-accent bg-transparent'}`}
                      onFocus={() => setHovered({ col, row })}
                      onBlur={() => setHovered(null)}
                      onMouseEnter={() => setHovered({ col, row })}
                      onMouseLeave={() => setHovered(null)}
                      onClick={() => {
                        editorRef.current?.insertAtCursor(
                          buildTableMarkdown(col, row),
                        )
                        setTablePickerOpen(false)
                        setHovered(null)
                      }}
                    />
                  )
                }),
              )}
            </div>
          </DropdownMenuContent>
        </DropdownMenu>
        <TooltipWrapper label="Code Block" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button"
            onClick={() =>
              editorRef.current?.insertWrappedText('```\n', '\n```')
            }
          >
            <Code className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Inline Code (Ctrl+`)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button"
            data-testid="format-inline-code-button"
            onClick={() => editorRef.current?.insertWrappedText('`')}
          >
            <Code2 className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Image / File" side="top" align="center">
          <Button
            data-testid="open-asset-manager-button"
            variant="ghost"
            size="icon"
            onClick={() =>
              openDialog(DIALOG_ASSET_MANAGER, {
                pageId,
                editorRef,
                isRenamingRef,
                onAssetVersionChange: assetChangedHandler,
              })
            }
            className="markdown-toolbar__button"
          >
            <Image className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <div className="markdown-toolbar__separator" />
        <TooltipWrapper label="Undo" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.undo()}
            className="markdown-toolbar__button"
            disabled={!canUndo}
          >
            <Undo className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Redo" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.redo()}
            className="markdown-toolbar__button"
            disabled={!canRedo}
          >
            <Redo className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        {!isMobile && (
          <>
            <div className="markdown-toolbar__separator" />
            <TooltipWrapper
              label={previewVisible ? 'Hide preview' : 'Show preview'}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                onClick={onTogglePreview}
                className="markdown-toolbar__button markdown-toolbar__button--desktop-only"
              >
                {!previewVisible ? (
                  <Eye className="markdown-toolbar__icon" />
                ) : (
                  <EyeOff className="markdown-toolbar__icon" />
                )}
              </Button>
            </TooltipWrapper>
          </>
        )}
      </div>
    </>
  )
}
