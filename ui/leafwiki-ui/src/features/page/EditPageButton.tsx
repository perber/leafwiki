import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Pencil } from 'lucide-react'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

export function EditPageButton({ path }: { path: string }) {
  const navigate = useNavigate()

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'e') {
        e.preventDefault()
        navigate(`/e/${path}`)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [navigate, path])

  return (
    <TooltipWrapper label="Edit page (Ctrl + e)" side="top" align="center">
      <Button
        data-testid="edit-page-button"
        className="h-8 w-8 rounded-full shadow-xs"
        variant="default"
        size="icon"
        onClick={() => navigate(`/e/${path}`)}
      >
        <Pencil size={20} />
      </Button>
    </TooltipWrapper>
  )
}
