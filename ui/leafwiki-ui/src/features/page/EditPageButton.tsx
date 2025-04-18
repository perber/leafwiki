import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Pencil } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function EditPageButton({ path }: { path: string }) {
  const navigate = useNavigate()
  return (
    <TooltipWrapper label="Edit page" side="top" align="center">
      <Button
        className="h-8 w-8 rounded-full shadow-sm"
        variant="default"
        size="icon"
        onClick={() => navigate(`/e/${path}`)}
      >
        <Pencil size={20} />
      </Button>
    </TooltipWrapper>
  )
}
