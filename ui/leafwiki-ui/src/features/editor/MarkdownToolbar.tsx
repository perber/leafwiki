import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DIALOG_ASSET_MANAGER, DIALOG_LINK_INSERT } from '@/lib/registries'
import { cn } from '@/lib/utils'
import { useIsMobile } from '@/lib/useIsMobile'
import { useDialogsStore } from '@/stores/dialogs'
import {
  Bold,
  ClipboardPaste,
  ClipboardType,
  Code,
  Code2,
  Eye,
  Image,
  Italic,
  Link,
  MoreHorizontal,
  Redo,
  Strikethrough,
  Table,
  Undo,
  WrapText,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useEditorStore } from '@/stores/editor'
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
  const { t } = useTranslation('editor')
  const openDialog = useDialogsStore((state) => state.openDialog)
  const lineWrap = useEditorStore((s) => s.lineWrap)
  const toggleLineWrap = useEditorStore((s) => s.toggleLineWrap)
  const autoSave = useEditorStore((s) => s.autoSave)
  const toggleAutoSave = useEditorStore((s) => s.toggleAutoSave)
  const autoSaveStatus = useEditorStore((s) => s.autoSaveStatus)
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
        <TooltipWrapper
          label={t('toolbar.boldTooltip')}
          side="top"
          align="center"
        >
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
        <TooltipWrapper
          label={t('toolbar.italicTooltip')}
          side="top"
          align="center"
        >
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
        {!isMobile && (
          <TooltipWrapper
            label={t('toolbar.strikethroughTooltip')}
            side="top"
            align="center"
          >
            <Button
              variant="ghost"
              size="icon"
              onClick={() => editorRef.current?.insertWrappedText('~~')}
              className="markdown-toolbar__button"
            >
              <Strikethrough className="markdown-toolbar__icon" />
            </Button>
          </TooltipWrapper>
        )}
        <TooltipWrapper
          label={t('toolbar.linkTooltip')}
          side="top"
          align="center"
        >
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
        {!isMobile && (
          <>
            <TooltipWrapper
              label={t('toolbar.heading1Tooltip')}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                onClick={() => editorRef.current?.insertHeading(1)}
              >
                H1
              </Button>
            </TooltipWrapper>
            <TooltipWrapper
              label={t('toolbar.heading2Tooltip')}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                onClick={() => editorRef.current?.insertHeading(2)}
              >
                H2
              </Button>
            </TooltipWrapper>
            <TooltipWrapper
              label={t('toolbar.heading3Tooltip')}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                onClick={() => editorRef.current?.insertHeading(3)}
              >
                H3
              </Button>
            </TooltipWrapper>
            <div className="markdown-toolbar__separator" />
            <DropdownMenu
              open={tablePickerOpen}
              onOpenChange={(open) => {
                setTablePickerOpen(open)
                if (!open) setHovered(null)
              }}
            >
              <TooltipWrapper
                label={t('toolbar.tableTooltip')}
                side="top"
                align="center"
              >
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
                  {hovered
                    ? t('toolbar.tableSizeLabel', {
                        col: hovered.col,
                        row: hovered.row,
                      })
                    : t('toolbar.tableSizeSelect')}
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
                          aria-label={t('toolbar.tableInsertAriaLabel', {
                            col,
                            row,
                          })}
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
          </>
        )}
        <TooltipWrapper
          label={t('toolbar.codeBlockTooltip')}
          side="top"
          align="center"
        >
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
        <TooltipWrapper
          label={t('toolbar.inlineCodeTooltip')}
          side="top"
          align="center"
        >
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
        <TooltipWrapper
          label={t('toolbar.imageTooltip')}
          side="top"
          align="center"
        >
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
        {!isMobile && (
          <>
            <div className="markdown-toolbar__separator" />
            <TooltipWrapper
              label={t('toolbar.pasteRichTooltip')}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                data-testid="paste-rich-button"
                onClick={() => editorRef.current?.pasteRich()}
              >
                <ClipboardType className="markdown-toolbar__icon" />
              </Button>
            </TooltipWrapper>
            <TooltipWrapper
              label={t('toolbar.pastePlainTooltip')}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                data-testid="paste-plain-button"
                onClick={() => editorRef.current?.pastePlain()}
              >
                <ClipboardPaste className="markdown-toolbar__icon" />
              </Button>
            </TooltipWrapper>
          </>
        )}
        <div className="markdown-toolbar__separator max-sm:hidden" />
        <TooltipWrapper
          label={t('toolbar.undoTooltip')}
          side="top"
          align="center"
        >
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
        <TooltipWrapper
          label={t('toolbar.redoTooltip')}
          side="top"
          align="center"
        >
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
          <TooltipWrapper
            label={t(
              lineWrap
                ? 'toolbar.lineWrapDisableTooltip'
                : 'toolbar.lineWrapEnableTooltip',
            )}
            side="top"
            align="center"
          >
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleLineWrap}
              className={cn('markdown-toolbar__button', {
                'markdown-toolbar__button--active': lineWrap,
              })}
              data-testid="toggle-line-wrap-button"
            >
              <WrapText className="markdown-toolbar__icon" />
            </Button>
          </TooltipWrapper>
        )}
        {isMobile && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="markdown-toolbar__button"
                aria-label={t('toolbar.moreOptionsAriaLabel')}
              >
                <MoreHorizontal className="markdown-toolbar__icon" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              onCloseAutoFocus={(e) => e.preventDefault()}
            >
              <DropdownMenuItem
                onSelect={() => editorRef.current?.insertWrappedText('~~')}
              >
                <Strikethrough size={14} />
                {t('toolbar.strikethrough')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => editorRef.current?.insertHeading(1)}
              >
                <span className="w-4 text-center text-xs font-bold">H1</span>
                {t('toolbar.heading1')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => editorRef.current?.insertHeading(2)}
              >
                <span className="w-4 text-center text-xs font-bold">H2</span>
                {t('toolbar.heading2')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => editorRef.current?.insertHeading(3)}
              >
                <span className="w-4 text-center text-xs font-bold">H3</span>
                {t('toolbar.heading3')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() =>
                  editorRef.current?.insertAtCursor(buildTableMarkdown(3, 3))
                }
              >
                <Table size={14} />
                {t('toolbar.insertTable')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => editorRef.current?.pasteRich()}>
                <ClipboardType size={14} />
                {t('toolbar.pasteRich')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => editorRef.current?.pastePlain()}
              >
                <ClipboardPaste size={14} />
                {t('toolbar.pastePlain')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem
                checked={lineWrap}
                onCheckedChange={(checked) => {
                  if (checked !== lineWrap) toggleLineWrap()
                }}
              >
                {t('toolbar.lineWrap')}
              </DropdownMenuCheckboxItem>
              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem
                checked={autoSave}
                onCheckedChange={(checked) => {
                  if (checked !== autoSave) toggleAutoSave()
                }}
              >
                {t('toolbar.autoSave')}
                {autoSaveStatus === 'paused' && (
                  <span className="text-muted-foreground ml-auto text-xs">
                    {t('toolbar.autoSavePaused')}
                  </span>
                )}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {!isMobile && (
          <>
            <div className="markdown-toolbar__separator" />
            <TooltipWrapper
              label={t(
                previewVisible
                  ? 'toolbar.hidePreviewTooltip'
                  : 'toolbar.showPreviewTooltip',
              )}
              side="top"
              align="center"
            >
              <Button
                variant="ghost"
                size="icon"
                onClick={onTogglePreview}
                className={cn(
                  'markdown-toolbar__button markdown-toolbar__button--desktop-only',
                  {
                    'markdown-toolbar__button--active': previewVisible,
                  },
                )}
              >
                <Eye className="markdown-toolbar__icon" />
              </Button>
            </TooltipWrapper>
          </>
        )}
      </div>
    </>
  )
}
