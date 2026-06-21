import { useState } from 'react'
import styles from './ChatListItem.module.css'
import ThreePoints from '@images/icons/three_points.svg?react'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@shared/ui/dropdown-menu'
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@shared/ui/alert-dialog'

interface ChatListItemProps {
    chatId: string
    title: string
    isActive?: boolean
    onClick?: () => void
    onDelete?: (chatId: string) => void
}

const ChatListItem = ({ chatId, title, isActive, onClick, onDelete }: ChatListItemProps) => {
    const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
    const className = [styles.item, isActive ? styles.itemActive : ''].join(' ')

    const handleDelete = () => {
        setIsDeleteDialogOpen(false)
        onDelete?.(chatId)
    }

    return (
        <>
            <div
                className={className}
                onClick={onClick}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onClick?.()}
            >
                <span className={styles.title}>{title}</span>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <button
                            className={styles.menu}
                            onClick={(e) => e.stopPropagation()}
                            aria-label="Меню"
                        >
                            <ThreePoints />
                        </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                        <DropdownMenuItem
                            variant="destructive"
                            onSelect={(e) => {
                                e.preventDefault()
                                setIsDeleteDialogOpen(true)
                            }}
                        >
                            Удалить диалог
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>

            <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
                <AlertDialogContent onClick={(e) => e.stopPropagation()}>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Вы точно хотите удалить этот диалог?</AlertDialogTitle>
                        <AlertDialogDescription>
                            Это действие нельзя отменить. Диалог будет удален навсегда.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel onClick={(e) => e.stopPropagation()}>Отмена</AlertDialogCancel>
                        <AlertDialogAction
                            onClick={(e) => {
                                e.stopPropagation()
                                handleDelete()
                            }}
                            variant="destructive"
                        >
                            Да
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </>
    )
}

export default ChatListItem