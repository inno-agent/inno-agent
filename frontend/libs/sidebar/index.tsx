import { useEffect, useState } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import styles from './Sidebar.module.css'
import Avatar from './ui/Avatar'
import ChatListItem from './ui/ChatListItem'
import Plus from '@images/icons/plus.svg?react'
import Loop from '@images/icons/loop.svg?react'
import Folder from '@images/icons/folder.svg?react'
import Logo from '@images/icons/logo.svg?react'
import ThreePoints from '@images/icons/three_points.svg?react'
import { chatsUpdatedEventName, listChats } from '@libs/chat/api/chatApi'
import type { ChatItem } from '@libs/chat/model/types'

const profileName = 'Фёдор Маркин'

export const Sidebar = () => {
    const navigate = useNavigate({ from: '/' })
    const chatId = useRouterState({
        select: (state) => {
            const nextChatId = (state.location.search as { chatId?: unknown }).chatId
            return typeof nextChatId === 'string' ? nextChatId : undefined
        },
    })
    const [chats, setChats] = useState<ChatItem[]>([])
    const [isLoading, setIsLoading] = useState(true)
    const [errorMessage, setErrorMessage] = useState<string | null>(null)

    useEffect(() => {
        let isMounted = true

        const loadChats = async (showLoader: boolean) => {
            if (showLoader) {
                setIsLoading(true)
            }
            setErrorMessage(null)

            try {
                const nextChats = await listChats()

                if (!isMounted) {
                    return
                }

                setChats(nextChats)
            } catch (error) {
                if (!isMounted) {
                    return
                }

                console.error('Failed to load chats', error)
                setErrorMessage('Не удалось загрузить чаты')
            } finally {
                if (isMounted && showLoader) {
                    setIsLoading(false)
                }
            }
        }

        const handleChatsUpdated = () => {
            void loadChats(false)
        }

        void loadChats(true)
        window.addEventListener(chatsUpdatedEventName, handleChatsUpdated)

        return () => {
            isMounted = false
            window.removeEventListener(chatsUpdatedEventName, handleChatsUpdated)
        }
    }, [])

    return (
        <aside className={styles.sidebar}>

            <div className={styles.header}>
                <Logo className={styles.logoIcon} />
                <span className={styles.logo}>INNOAGENT</span>
            </div>

            <div className={styles.divider} />

            <nav className={styles.nav}>
                <button
                    className={styles.navItem}
                    onClick={() =>
                        navigate({
                            to: '/',
                            search: { chatId: undefined },
                        })
                    }
                >
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
                {isLoading && <span className={styles.sectionTitle}>Загрузка...</span>}
                {!isLoading && errorMessage && <span className={styles.sectionTitle}>{errorMessage}</span>}
                {!isLoading && !errorMessage && chats.length === 0 && (
                    <span className={styles.sectionTitle}>Чатов пока нет</span>
                )}
                {!isLoading &&
                    !errorMessage &&
                    chats.map((chat) => (
                        <ChatListItem
                            key={chat.id}
                            title={chat.title || chat.last_message || 'Новый чат'}
                            isActive={chat.id === chatId}
                            onClick={() =>
                                navigate({
                                    to: '/',
                                    search: { chatId: chat.id },
                                })
                            }
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
