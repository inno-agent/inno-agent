import styles from './Avatar.module.css';

interface AvatarProps {
    name: string
    src?: string
    onClick?: () => void
}

const Avatar = ({ name, src, onClick }: AvatarProps) => {
    const content = src ? <img src={src} alt={name[0]} /> : name[0]

    if (onClick) {
        return <button className={styles.avatar} onClick={onClick}>{content}</button>
    }
    return <div className={styles.avatar}>{content}</div>
}

export default Avatar
