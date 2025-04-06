import { Button } from '@/components/ui/button'
import { Pencil } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function EditPageButton({ path }: { path: string }) {
  const navigate = useNavigate()
  return (
    <Button
      className="ml-2"
      size="sm"
      variant="outline"
      onClick={() => navigate(`/e/${path}`)}
    >
      <Pencil className="mr-1 h-4 w-4" />
      Edit
    </Button>
  )
}
