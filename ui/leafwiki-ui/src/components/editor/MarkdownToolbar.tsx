import { Button } from '@/components/ui/button'
import { AssetManager } from '@/features/page/AssetManager'
import { DialogDescription, DialogTitle } from '@radix-ui/react-dialog'
import { Bold, Code, Image, Italic, Link, Strikethrough, Table } from 'lucide-react'
import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader } from '../ui/dialog'
import { MarkdownEditorRef } from './MarkdownEditor'

type Props = {
  editorRef: React.RefObject<MarkdownEditorRef>
  pageId: string
}

export default function MarkdownToolbar({ editorRef, pageId }: Props) {
  const [assetModalOpen, setAssetModalOpen] = useState(false)

  const toolbarButtonStyle = "text-white hover:text-white hover:bg-zinc-800"

  const insertHeading = (level: 1 | 2 | 3) => {
    const prefix = '#'.repeat(level) + ' '
    editorRef.current?.insertWrappedText(prefix, '')
  }

  const tableMarkdown = `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`

  return (
    <>
      <div className="sticky top-0 z-10 flex gap-1.5 border-b border-zinc-700 bg-zinc-900 p-2 shadow-sm">
        <Button variant="ghost" size="icon" onClick={() => editorRef.current?.insertWrappedText('**')} className={toolbarButtonStyle}>
          <Bold className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" onClick={() => editorRef.current?.insertWrappedText('_')} className={toolbarButtonStyle}>
          <Italic className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" onClick={() => editorRef.current?.insertWrappedText('~~')} className={toolbarButtonStyle}>
          <Strikethrough className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" className={toolbarButtonStyle} onClick={() => editorRef.current?.insertWrappedText('[', '](https://example.com)')}>
          <Link className="w-4 h-4" />
        </Button>
        <div className="mx-1 h-5 w-px bg-white/30 self-center" />
        <Button
          variant="ghost"
          size="icon"
          className={toolbarButtonStyle}
          onClick={() => insertHeading(1)}
        >
          H1
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className={toolbarButtonStyle}
          onClick={() => insertHeading(2)}
        >
          H2
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className={toolbarButtonStyle}
          onClick={() => insertHeading(3)}
        >
          H3
        </Button>
        <div className="mx-1 h-5 w-px bg-white/30 self-center" />
        <Button variant="ghost" size="icon" onClick={() => editorRef.current?.insertAtCursor(tableMarkdown)} className={toolbarButtonStyle}>
          <Table className="w-4 h-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className={toolbarButtonStyle}
          onClick={() => editorRef.current?.insertWrappedText('```\n', '\n```')}
        >
          <Code className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" onClick={() => setAssetModalOpen(true)} className={toolbarButtonStyle}>
          <Image className="w-4 h-4" />
        </Button>
      </div >

      <Dialog open={assetModalOpen} onOpenChange={setAssetModalOpen}>
        <DialogContent
          className="max-w-2xl"
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.stopPropagation()
              e.preventDefault()
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>Add Asset</DialogTitle>
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
          />
        </DialogContent>
      </Dialog>
    </>
  )
}
