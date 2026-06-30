"use client";

import { type PropsWithChildren, useEffect, useMemo, useState, type FC } from "react";
import { XIcon, PlusIcon, FileText } from "lucide-react";
import {
  AttachmentPrimitive,
  ComposerPrimitive,
  MessagePrimitive,
  useAuiState,
  useAui,
} from "@assistant-ui/react";
import { useShallow } from "zustand/shallow";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@shared/ui/tooltip";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogTrigger,
} from "@shared/ui/dialog";
import { Avatar, AvatarImage, AvatarFallback } from "@shared/ui/avatar";
import { TooltipIconButton } from "@libs/chat/ui/common/TooltipIconButton";
import { cn } from "@shared/lib/utils";
import { attachmentStyles as styles } from "./attachments.styles";

const useFileSrc = (file: File | undefined) => {
  const src = useMemo(() => {
    if (!file) return undefined;
    return URL.createObjectURL(file);
  }, [file]);

  useEffect(() => {
    if (!src) return;

    return () => {
      URL.revokeObjectURL(src);
    };
  }, [src]);

  return src;
};

const useAttachmentSrc = () => {
  const { file, src } = useAuiState(
    useShallow((s): { file?: File; src?: string } => {
      if (s.attachment.type !== "image") return {};
      if (s.attachment.file) return { file: s.attachment.file };
      const src = s.attachment.content?.filter((c) => c.type === "image")[0]
        ?.image;
      if (!src) return {};
      return { src };
    }),
  );

  return useFileSrc(file) ?? src;
};

type AttachmentPreviewProps = {
  src: string;
};

const AttachmentPreview: FC<AttachmentPreviewProps> = ({ src }) => {
  const [isLoaded, setIsLoaded] = useState(false);
  return (
    <img
      src={src}
      alt="Attachment preview"
      className={cn(
        styles.previewImage,
        isLoaded ? styles.previewImageLoaded : styles.previewImageLoading,
      )}
      onLoad={() => setIsLoaded(true)}
    />
  );
};

const AttachmentPreviewDialog: FC<PropsWithChildren> = ({ children }) => {
  const src = useAttachmentSrc();

  if (!src) return children;

  return (
    <Dialog>
      <DialogTrigger
        className={styles.previewTrigger}
        asChild
      >
        {children}
      </DialogTrigger>
      <DialogContent className={styles.previewDialogContent}>
        <DialogTitle className="aui-sr-only sr-only">
          Image Attachment Preview
        </DialogTitle>
        <div className={styles.previewWrapper}>
          <AttachmentPreview src={src} />
        </div>
      </DialogContent>
    </Dialog>
  );
};

const AttachmentThumb: FC = () => {
  const src = useAttachmentSrc();

  return (
    <Avatar className={styles.avatar}>
      <AvatarImage
        src={src}
        alt="Attachment preview"
        className={styles.avatarImage}
      />
      <AvatarFallback>
        <FileText className={styles.fallbackIcon} />
      </AvatarFallback>
    </Avatar>
  );
};

const AttachmentUI: FC = () => {
  const aui = useAui();
  const isComposer = aui.attachment.source !== "message";

  const isImage = useAuiState((s) => s.attachment.type === "image");
  const typeLabel = useAuiState((s) => {
    const type = s.attachment.type;
    switch (type) {
      case "image":
        return "Image";
      case "document":
        return "Document";
      case "file":
        return "File";
      default:
        return type;
    }
  });

  return (
    <Tooltip>
      <AttachmentPrimitive.Root
        className={cn(
          styles.root,
          isImage && styles.rootImage,
        )}
      >
        <AttachmentPreviewDialog>
          <TooltipTrigger asChild>
            <div
              className={styles.tile}
              role="button"
              tabIndex={0}
              aria-label={`${typeLabel} attachment`}
            >
              <AttachmentThumb />
            </div>
          </TooltipTrigger>
        </AttachmentPreviewDialog>
        {isComposer && <AttachmentRemove />}
      </AttachmentPrimitive.Root>
      <TooltipContent side="top">
        <AttachmentPrimitive.Name />
      </TooltipContent>
    </Tooltip>
  );
};

const AttachmentRemove: FC = () => {
  return (
    <AttachmentPrimitive.Remove asChild>
      <TooltipIconButton
        tooltip="Remove file"
        className={styles.tileRemove}
        side="top"
      >
        <XIcon className={styles.removeIcon} />
      </TooltipIconButton>
    </AttachmentPrimitive.Remove>
  );
};

export const UserMessageAttachments: FC = () => {
  return (
    <div className={styles.userMessageContainer}>
      <MessagePrimitive.Attachments>
        {() => <AttachmentUI />}
      </MessagePrimitive.Attachments>
    </div>
  );
};

export const ComposerAttachments: FC = () => {
  return (
    <div className={styles.composerContainer}>
      <ComposerPrimitive.Attachments>
        {() => <AttachmentUI />}
      </ComposerPrimitive.Attachments>
    </div>
  );
};

export const ComposerAddAttachment: FC = () => {
  return (
    <ComposerPrimitive.AddAttachment asChild>
      <TooltipIconButton
        tooltip="Add Attachment"
        side="bottom"
        variant="ghost"
        size="icon"
        className={styles.addButton}
        aria-label="Add Attachment"
      >
        <PlusIcon className={styles.addIcon} />
      </TooltipIconButton>
    </ComposerPrimitive.AddAttachment>
  );
};
