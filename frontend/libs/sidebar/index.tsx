import { useState } from 'react'
import styles from './Sidebar.module.css'
import Avatar from './ui/Avatar'
import ChatListItem from './ui/ChatListItem'
import { mockChats } from './model/mockChats'
import Plus from '@images/icons/plus.svg?react'
import Loop from '@images/icons/loop.svg?react'
import Folder from '@images/icons/folder.svg?react'
import Logo from '@images/icons/logo.svg?react'
import ThreePoints from '@images/icons/three_points.svg?react'

const profileName = 'Фёдор Маркин'

export const Sidebar = () => {
    const [activeChatId, setActiveChatId] = useState<number | null>(null)

    return (
        <aside className={styles.sidebar}>

            <div className={styles.header}>
                <Logo className={styles.logoIcon} />
                <span className={styles.logo}>INNOAGENT</span>
            </div>

            <div className={styles.divider} />

            <nav className={styles.nav}>
                <button className={styles.navItem}>
                    <span className={styles.navIcon}><Plus /></span>
                    Новый чат
                </button>
                <button className={styles.navItem}>
                    <span className={styles.navIcon}><Loop /></span>
                    Искать чат
                </button>
                <button className={styles.navItem}>
                    <span className={styles.navIcon}><Folder /></span>
                    Проекты
                </button>
            </nav>

            <div className={styles.divider} />

            <div className={styles.chatList}>
                <span className={styles.sectionTitle}>Недавние</span>
                {mockChats.map(chat => (
                    <ChatListItem
                        key={chat.id}
                        title={chat.title}
                        isActive={chat.id === activeChatId}
                        onClick={() => setActiveChatId(chat.id)}
                    />
                ))}
            </div>

            <div className={styles.divider} />

            <div
                className={styles.profile}
                role="button"
                tabIndex={0}
                onClick={() => {}}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && e.currentTarget.click()}
            >
                <Avatar name={profileName} />
                <span className={styles.profileName}>{profileName}</span>
                <button
                    className={styles.profileMenu}
                    onClick={(e) => e.stopPropagation()}
                >
                    <ThreePoints />
                </button>
            </div>

        </aside>
    )
}
