import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import styles from './Sidebar.module.scss'
import Avatar from './ui/Avatar'
import ChatListItem from './ui/ChatListItem'
import Plus from '@images/icons/plus.svg?react'
import Loop from '@images/icons/loop.svg?react'
import Folder from '@images/icons/folder.svg?react'
import Logo from '@images/icons/logo.svg?react'
import ThreePoints from '@images/icons/three_points.svg?react'
import { chatsUpdatedEventName, listChats, deleteChat } from '@libs/chat/api/chatApi'
import type { ChatItem } from '@libs/chat/model/types'
import { Popover, PopoverContent, PopoverTrigger } from '@shared/ui/popover'
import { AccountMenu } from '@libs/settings/ui/AccountMenu'
import { SettingsDialog } from '@libs/settings/ui/SettingsDialog'
import { getCurrentUser } from '@libs/settings/api/settingsApi'
import { useAuth } from '@libs/auth/useAuth'

const profileName = 'Фёдор Маркин'

interface SidebarProps {
    isOpen?: boolean
    onClose?: () => void
}

export const Sidebar = ({ isOpen = true, onClose }: SidebarProps) => {
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
    const [email, setEmail] = useState('')
    const [accountMenuOpen, setAccountMenuOpen] = useState(false)
    const [settingsOpen, setSettingsOpen] = useState(false)
    const { clearSession } = useAuth()
    const { t } = useTranslation()

    useEffect(() => {
        if (typeof window !== 'undefined' && window.innerWidth <= 768) {
            return
        }
    }, [isOpen])

    useEffect(() => {
        getCurrentUser().then((user) => setEmail(user.email))
    }, [])

    const handleLogout = () => {
        setAccountMenuOpen(false)
        clearSession()
    }

    const handleDeleteChat = async (deletedChatId: string) => {
        try {
            await deleteChat(deletedChatId)

            if (deletedChatId === chatId) {
                await navigate({
                    to: '/',
                    search: { chatId: undefined },
                })
            }
        } catch (error) {
            console.error('Failed to delete chat', error)
            setErrorMessage(t('sidebar.deleteError'))
        }
    }

    const handleNavigate = (action: () => void) => {
        action()
        if (onClose) {
            onClose()
        }
    }

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
                setErrorMessage(t('sidebar.loadError'))
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
    }, [t])

    return (
        <>
            {isOpen && <div className={styles['sidebar__overlay']} onClick={onClose} />}
            <aside className={`${styles.sidebar} ${isOpen ? styles['sidebar--open'] : ''}`}>

                <div className={styles['sidebar__header']}>
                    <Logo className={styles['sidebar__logo-icon']} />
                    <span className={styles['sidebar__logo']}>INNOAGENT</span>
                </div>

                <div className={styles['sidebar__divider']} />

                <nav className={styles['sidebar__nav']}>
                    <button
                        className={styles['sidebar__nav-item']}
                        onClick={() =>
                            handleNavigate(() =>
                                navigate({
                                    to: '/',
                                    search: { chatId: undefined },
                                })
                            )
                        }
                    >
                        <span className={styles['sidebar__nav-icon']}><Plus /></span>
                        {t('sidebar.newChat')}
                    </button>
                    <button className={styles['sidebar__nav-item']}>
                        <span className={styles['sidebar__nav-icon']}><Loop /></span>
                        {t('sidebar.searchChat')}
                    </button>
                    <button className={styles['sidebar__nav-item']}>
                        <span className={styles['sidebar__nav-icon']}><Folder /></span>
                        {t('sidebar.projects')}
                    </button>
                </nav>

                <div className={styles['sidebar__divider']} />

                <div className={styles['sidebar__chat-list']}>
                    <span className={styles['sidebar__section-title']}>{t('sidebar.recent')}</span>
                    {isLoading && <span className={styles['sidebar__section-title']}>{t('sidebar.loading')}</span>}
                    {!isLoading && errorMessage && <span className={styles['sidebar__section-title']}>{errorMessage}</span>}
                    {!isLoading && !errorMessage && chats.length === 0 && (
                        <span className={styles['sidebar__section-title']}>{t('sidebar.noChats')}</span>
                    )}
                    {!isLoading &&
                        !errorMessage &&
                        chats.map((chat) => (
                            <ChatListItem
                                key={chat.id}
                                chatId={chat.id}
                                title={chat.title || chat.last_message || t('sidebar.newChat')}
                                isActive={chat.id === chatId}
                                onClick={() =>
                                    handleNavigate(() =>
                                        navigate({
                                            to: '/',
                                            search: { chatId: chat.id },
                                        })
                                    )
                                }
                                onDelete={handleDeleteChat}
                            />
                        ))}
                </div>

                <div className={styles['sidebar__divider']} />

                <Popover open={accountMenuOpen} onOpenChange={setAccountMenuOpen}>
                    <PopoverTrigger asChild>
                        <div
                            className={styles['sidebar__profile']}
                            role="button"
                            tabIndex={0}
                            onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && e.currentTarget.click()}
                        >
                            <Avatar name={profileName} />
                            <span className={styles['sidebar__profile-name']}>{profileName}</span>
                            <button
                                className={styles['sidebar__profile-menu']}
                                onClick={(e) => e.stopPropagation()}
                            >
                                <ThreePoints />
                            </button>
                        </div>
                    </PopoverTrigger>
                    <PopoverContent
                        side="top"
                        align="start"
                        alignOffset={20}
                        className="p-0 border-none bg-transparent shadow-none"
                    >
                        <AccountMenu
                            email={email}
                            onOpenSettings={() => {
                                setAccountMenuOpen(false)
                                setSettingsOpen(true)
                            }}
                            onLogout={handleLogout}
                        />
                    </PopoverContent>
                </Popover>

                <SettingsDialog
                    open={settingsOpen}
                    onOpenChange={setSettingsOpen}
                    email={email}
                    onLogout={handleLogout}
                />

            </aside>
        </>
    )
}
