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
        <div
            className={className}
            onClick={onClick}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onClick?.()}
        >
            <span className={styles.title}>{title}</span>
            <button
                className={styles.menu}
                onClick={(e) => { e.stopPropagation(); onMenuClick?.(); }}
            >
                <ThreePoints />
            </button>
        </div>
    )
}

export default ChatListItem