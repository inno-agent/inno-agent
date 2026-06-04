import styles from './ChatListItem.module.css';
import ThreePoints from '../../shared/ui/icons/three_points.svg?react';

interface ChatListItemProps {
    title: string
    isActive?: boolean
    onClick?: () => void
    onMenuClick?: () => void
}

const ChatListItem = ({ title, isActive, onClick, onMenuClick }: ChatListItemProps) => {
    const className = [styles.item, isActive ? styles.itemActive : ''].join(' ')

    return (
        <button className={className} onClick={onClick}>
            <span className={styles.title}>{title}</span>
            <button className={styles.menu} onClick={onMenuClick}>
                <ThreePoints />
            </button>
        </button>
    )
}

export default ChatListItem