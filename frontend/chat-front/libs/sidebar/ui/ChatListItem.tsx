import styles from './ChatListItem.module.css';
import ThreePoints from '@images/icons/three_points.svg?react';

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
                {/*Lorem ipsum dolor sit amet, consectetur adipisicing elit. Ad aut beatae blanditiis consectetur consequatur cupiditate ducimus esse exercitationem hic laborum minima minus molestiae molestias odio omnis placeat, quae quam quas quis quo quos reiciendis vel veritatis vitae voluptates? Distinctio magni omnis quae repellat tenetur vero. Aliquid at expedita hic praesentium.*/}
            </button>
        </div>
    )
}

export default ChatListItem