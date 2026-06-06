import { BranchPickerPrimitive } from '@assistant-ui/react'
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { TooltipIconButton } from '@libs/chat/ui/common/TooltipIconButton'
import { cn } from '@shared/lib/utils'

export const BranchPicker = ({
    className,
    ...rest
}: BranchPickerPrimitive.Root.Props) => {
    return (
        <BranchPickerPrimitive.Root
            hideWhenSingleBranch
            className={cn(
                'aui-branch-picker-root text-muted-foreground -ms-2 me-2 inline-flex items-center text-xs',
                className,
            )}
            {...rest}
        >
            <BranchPickerPrimitive.Previous asChild>
                <TooltipIconButton tooltip="Previous">
                    <ChevronLeftIcon />
                </TooltipIconButton>
            </BranchPickerPrimitive.Previous>
            <span className="aui-branch-picker-state font-medium">
                <BranchPickerPrimitive.Number /> / <BranchPickerPrimitive.Count />
            </span>
            <BranchPickerPrimitive.Next asChild>
                <TooltipIconButton tooltip="Next">
                    <ChevronRightIcon />
                </TooltipIconButton>
            </BranchPickerPrimitive.Next>
        </BranchPickerPrimitive.Root>
    )
}
