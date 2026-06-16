import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@shared/lib/utils'

type StaticMarkdownProps = {
    content: string
    className?: string
}

export const StaticMarkdown = ({ content, className }: StaticMarkdownProps) => {
    return (
        <div className={cn('aui-md text-foreground leading-relaxed', className)}>
            <Markdown
                remarkPlugins={[remarkGfm]}
                components={{
                    h1: ({ className: headingClassName, ...props }) => (
                        <h1
                            className={cn(
                                'aui-md-h1 mb-2 scroll-m-20 text-base font-semibold first:mt-0 last:mb-0',
                                headingClassName,
                            )}
                            {...props}
                        />
                    ),
                    h2: ({ className: headingClassName, ...props }) => (
                        <h2
                            className={cn(
                                'aui-md-h2 mt-3 mb-1.5 scroll-m-20 text-sm font-semibold first:mt-0 last:mb-0',
                                headingClassName,
                            )}
                            {...props}
                        />
                    ),
                    p: ({ className: paragraphClassName, ...props }) => (
                        <p
                            className={cn(
                                'aui-md-p my-2.5 leading-normal first:mt-0 last:mb-0',
                                paragraphClassName,
                            )}
                            {...props}
                        />
                    ),
                    ul: ({ className: listClassName, ...props }) => (
                        <ul
                            className={cn(
                                'aui-md-ul marker:text-muted-foreground my-2 ms-4 list-disc [&>li]:mt-1',
                                listClassName,
                            )}
                            {...props}
                        />
                    ),
                    code: ({ className: codeClassName, ...props }) => (
                        <code
                            className={cn(
                                'aui-md-inline-code border-border/50 bg-muted/50 rounded-md border px-1.5 py-0.5 font-mono text-[0.85em]',
                                codeClassName,
                            )}
                            {...props}
                        />
                    ),
                }}
            >
                {content}
            </Markdown>
        </div>
    )
}
