import { Button } from '@/components/ui/button'
import { Pencil } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function EditPageButton({ path }: { path: string }) {
  const navigate = useNavigate()
  return (
    <Button
      className='rounded-full shadow-sm'
      variant="default"
      size="icon"
      onClick={() => navigate(`/e/${path}`)}
    >
      <Pencil size={20} />
    </Button>
  )
}
