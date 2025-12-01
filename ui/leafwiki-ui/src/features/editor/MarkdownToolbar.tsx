import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { DIALOG_ASSET_MANAGER } from '@/lib/registries'
import { useIsMobile } from '@/lib/useIsMobile'
import { useDialogsStore } from '@/stores/dialogs'
import {
  Bold,
  Code,
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

  const tableMarkdown = `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`

  return (
    <>
      <div className="markdown-toolbar">
        <TooltipWrapper label="Bold (Ctrl+B)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('**')}
            className="markdown-toolbar__button"
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
          >
            <Italic className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Strike through" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('~~')}
            className="markdown-toolbar__button"
          >
            <Strikethrough className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Link" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className="markdown-toolbar__button"
            onClick={() =>
              editorRef.current?.insertWrappedText(
                '[',
                '](https://example.com)',
              )
            }
          >
            <Link className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <div className="markdown-toolbar__separator" />
        <TooltipWrapper
          label="Headline - H1 (Ctrl + Alt + 1)"
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
          label="Headline - H2 (Ctrl + Alt + 2)"
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
          label="Headline - H3 (Ctrl + Alt + 3)"
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
        <TooltipWrapper label="Table" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertAtCursor(tableMarkdown)}
            className="markdown-toolbar__button"
          >
            <Table className="markdown-toolbar__icon" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Codeblock" side="top" align="center">
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
        <TooltipWrapper label="Add Image or File" side="top" align="center">
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
          <><div className="markdown-toolbar__separator" />
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
            </TooltipWrapper></>
        )}
      </div>
    </>
  )
}
