"use client";

import {
  useState,
  useRef,
  useEffect,
  forwardRef,
  useImperativeHandle,
} from "react";
import { Textarea } from "@/components/ui/textarea";
import { Response } from "@/components/ai-elements/response";
import { cn } from "@/lib/utils.ts";

interface EditableMessageProps {
  content: string;
  isEditing: boolean;
  onSave: (newContent: string) => void;
  onCancel: () => void;
  disabled?: boolean;
  className?: string;
}

export interface EditableMessageRef {
  triggerSave: () => void;
}

export const EditableMessage = forwardRef<
  EditableMessageRef,
  EditableMessageProps
>(
  (
    { content, isEditing, onSave, onCancel, disabled = false, className },
    ref,
  ) => {
    const [editContent, setEditContent] = useState(content);
    const textareaRef = useRef<HTMLTextAreaElement>(null);

    // Update edit content when content prop changes
    useEffect(() => {
      setEditContent(content);
    }, [content]);

    // Focus and select all text when entering edit mode
    useEffect(() => {
      if (isEditing && textareaRef.current) {
        textareaRef.current.focus();
        textareaRef.current.select();
      }
    }, [isEditing]);

    const handleSave = () => {
      const trimmedContent = editContent.trim();
      if (trimmedContent && trimmedContent !== content) {
        onSave(trimmedContent);
      } else {
        onCancel();
      }
    };

    useImperativeHandle(ref, () => ({
      triggerSave: handleSave,
    }));

    const handleCancel = () => {
      setEditContent(content);
      onCancel();
    };

    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        handleSave();
      } else if (e.key === "Escape") {
        e.preventDefault();
        handleCancel();
      }
    };

    if (isEditing) {
      return (
        <Textarea
          ref={textareaRef}
          value={editContent}
          onChange={(e) => setEditContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter your message..."
          className={cn("min-h-[100px] resize-none", className)}
          disabled={disabled}
        />
      );
    }

    return <Response className={className}>{content}</Response>;
  },
);

EditableMessage.displayName = "EditableMessage";
