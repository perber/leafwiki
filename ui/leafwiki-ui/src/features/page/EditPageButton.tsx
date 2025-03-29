import { Button } from "@/components/ui/button"
import { Pencil } from "lucide-react"
import { useNavigate } from "react-router-dom"

export function EditPageButton({ path }: { path: string }) {
    const navigate = useNavigate()
    return (
        <Button
            className="ml-2"
            size="sm"
            variant="outline"
            onClick={() => navigate(`/e/${path}`)}
        >
            <Pencil className="w-4 h-4 mr-1" />
            Edit
        </Button>
    )
}