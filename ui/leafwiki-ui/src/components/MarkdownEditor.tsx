import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

export default function MarkdownEditor({
  value = '',
  onChange,
}: {
  value?: string
  onChange: (newValue: string) => void
}) {
  return (
    <div className="flex h-full gap-4">
      <textarea
        className="w-1/2 resize-none rounded border border-gray-300 p-4 font-mono focus:outline-none focus:ring-2 focus:ring-green-500"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Write in Markdown..."
      />

      <div className="prose prose-lg w-1/2 max-w-none overflow-auto rounded border border-gray-200 bg-white p-4 leading-relaxed [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{value}</ReactMarkdown>
      </div>
    </div>
  )
}
