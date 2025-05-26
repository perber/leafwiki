import { Button } from '@/components/ui/button'
import { Bold, Image, Italic } from 'lucide-react'
import { MarkdownEditorRef } from './MarkdownEditor'

type Props = {
  editorRef: React.RefObject<MarkdownEditorRef>
}

export default function MarkdownToolbar({ editorRef }: Props) {

  return (
    <div className="flex gap-1 border-b px-2 py-1 bg-muted">
      <Button variant="ghost" size="sm" onClick={() => editorRef.current?.insertWrappedText("**")}>
        <Bold className="w-4 h-4" />
      </Button>
      <Button variant="ghost" size="sm" onClick={() => editorRef.current?.insertWrappedText('_')}>
        <Italic className="w-4 h-4" />
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={() =>
          editorRef.current?.insertAtCursor('![Alt text](https://)')
        }
      >
        <Image className="w-4 h-4" />
      </Button>
    </div>
  )
}
