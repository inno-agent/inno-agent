import styles from './Avatar.module.css';

interface AvatarProps {
    name: string
    src?: string
    onClick?: () => void
}

const Avatar = ({ name, src, onClick }: AvatarProps) => {
    const className = styles.avatar

    return (
        <button className={className} onClick={onClick}>
            {src ? <img src={src} alt={name[0]} /> : name[0]}
        </button>
    )
}

export default Avatar