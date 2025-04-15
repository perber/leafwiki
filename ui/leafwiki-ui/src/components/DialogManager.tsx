import { MovePageDialog } from "@/features/page/MovePageDialog"
import { SortPagesDialog } from "@/features/page/SortPagesDialog"
import { useDialogsStore } from "@/stores/dialogs"

export function DialogManger() {
    const dialogType = useDialogsStore((state) => state.dialogType)
    const dialogProps = useDialogsStore((state) => state.dialogProps)

    console.log("DialogManger", dialogType, dialogProps)

    return (
        <>
            {dialogType === "sort" && (
                <SortPagesDialog
                    {...dialogProps}
                />)
            }
            {dialogType === "move" && (
                <MovePageDialog
                    {...dialogProps}
                />)
            }
        </>
    )
}