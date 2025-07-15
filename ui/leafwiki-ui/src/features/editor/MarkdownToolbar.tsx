import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Bold,
  Code,
  Image,
  Italic,
  Link,
  Redo,
  Strikethrough,
  Table,
  Undo,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { AssetManager } from '../assets/AssetManager'
import { MarkdownEditorRef } from './MarkdownEditor'

type Props = {
  editorRef: React.RefObject<MarkdownEditorRef>
  onAssetVersionChange?: (version: number) => void
  pageId: string
}

export default function MarkdownToolbar({
  editorRef,
  onAssetVersionChange,
  pageId,
}: Props) {
  const [assetModalOpen, setAssetModalOpen] = useState(false)
  const [canUndo, setCanUndo] = useState(false)
  const [canRedo, setCanRedo] = useState(false)
  const isRenamingRef = useRef(false)

  const toolbarButtonStyle = 'text-white hover:text-white hover:bg-zinc-800'

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
  useEffect(() => {
    if (onAssetVersionChange) {
      onAssetVersionChange(Date.now() || 0)
    }
  }, [onAssetVersionChange, assetModalOpen])

  const tableMarkdown = `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`

  return (
    <>
      <div className="sticky top-0 z-10 flex flex-wrap gap-1.5 border-b border-zinc-700 bg-zinc-900 p-2 shadow-xs">
        <TooltipWrapper label="Bold (Ctrl+B)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('**')}
            className={toolbarButtonStyle}
          >
            <Bold className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Italic (Ctrl+I)" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('_')}
            className={toolbarButtonStyle}
          >
            <Italic className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Strike through" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertWrappedText('~~')}
            className={toolbarButtonStyle}
          >
            <Strikethrough className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Link" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className={toolbarButtonStyle}
            onClick={() =>
              editorRef.current?.insertWrappedText(
                '[',
                '](https://example.com)',
              )
            }
          >
            <Link className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <div className="mx-1 h-5 w-px self-center bg-white/30" />
        <TooltipWrapper
          label="Headline - H1 (Ctrl + Alt + 1)"
          side="top"
          align="center"
        >
          <Button
            variant="ghost"
            size="icon"
            className={toolbarButtonStyle + ' max-md:hidden'}
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
            className={toolbarButtonStyle + ' max-md:hidden'}
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
            className={toolbarButtonStyle + ' max-md:hidden'}
            onClick={() => editorRef.current?.insertHeading(3)}
          >
            H3
          </Button>
        </TooltipWrapper>
        <div className="mx-1 h-5 w-px self-center bg-white/30 max-md:hidden" />
        <TooltipWrapper label="Table" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.insertAtCursor(tableMarkdown)}
            className={toolbarButtonStyle}
          >
            <Table className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Codeblock" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            className={toolbarButtonStyle}
            onClick={() =>
              editorRef.current?.insertWrappedText('```\n', '\n```')
            }
          >
            <Code className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Add Image or File" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setAssetModalOpen(true)}
            className={toolbarButtonStyle}
          >
            <Image className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <div className="mx-1 h-5 w-px self-center bg-white/30" />
        <TooltipWrapper label="Undo" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.undo()}
            className={toolbarButtonStyle}
            disabled={!canUndo}
          >
            <Undo className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Redo" side="top" align="center">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => editorRef.current?.redo()}
            className={toolbarButtonStyle}
            disabled={!canRedo}
          >
            <Redo className="h-4 w-4" />
          </Button>
        </TooltipWrapper>
      </div>

      <Dialog open={assetModalOpen} onOpenChange={setAssetModalOpen}>
        <DialogContent
          className="max-w-2xl"
          onEscapeKeyDown={(e) => {
            if (isRenamingRef.current) {
              e.preventDefault()
            } else {
              setAssetModalOpen(false)
              e.preventDefault()
              e.stopPropagation()
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>Asset Manager</DialogTitle>
            <DialogDescription>
              Upload or select an asset to insert into the page.
            </DialogDescription>
          </DialogHeader>
          <AssetManager
            pageId={pageId}
            onInsert={(md) => {
              editorRef.current?.insertAtCursor(md)
              setAssetModalOpen(false)
            }}
            isRenamingRef={isRenamingRef}
          />
        </DialogContent>
      </Dialog>
    </>
  )
}
