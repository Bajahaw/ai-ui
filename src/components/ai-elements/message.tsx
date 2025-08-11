import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import type { ComponentProps, HTMLAttributes } from "react";
import { cn } from "@/lib/utils";
import type { UIMessage } from "ai";

export type MessageProps = HTMLAttributes<HTMLDivElement> & {
  from: UIMessage["role"];
  status?: "success" | "error" | "pending";
};

export const Message = ({
  className,
  from,
  status,
  ...props
}: MessageProps) => (
  <div
    className={cn(
      "group flex w-full items-start gap-2 py-6",
      from === "user" ? "is-user justify-end" : "is-assistant justify-start",
      from === "user" ? "[&>div]:max-w-[80%]" : "[&>div]:max-w-full",
      status === "error" && "is-error",
      className,
    )}
    {...props}
  />
);

export type MessageContentProps = HTMLAttributes<HTMLDivElement>;

export const MessageContent = ({
  children,
  className,
  ...props
}: MessageContentProps) => (
  <div
    className={cn(
      "flex flex-col gap-2 text-sm text-foreground overflow-hidden leading-relaxed",
      "group-[.is-user]:bg-primary group-[.is-user]:text-primary-foreground group-[.is-user]:rounded-lg group-[.is-user]:px-4 group-[.is-user]:py-3",
      "group-[.is-assistant]:bg-transparent group-[.is-assistant]:text-foreground group-[.is-assistant]:px-0 group-[.is-assistant]:py-0",
      "group-[.is-error]:bg-destructive/10 group-[.is-error]:border group-[.is-error]:border-destructive/20 group-[.is-error]:rounded-lg group-[.is-error]:px-4 group-[.is-error]:py-3",
      className,
    )}
    {...props}
  >
    <div className="group-[.is-user]:dark space-y-4">{children}</div>
  </div>
);

export type MessageAvatarProps = ComponentProps<typeof Avatar> & {
  src: string;
  name?: string;
};

export const MessageAvatar = ({
  src,
  name,
  className,
  ...props
}: MessageAvatarProps) => (
  <Avatar className={cn("size-8 ring-1 ring-border", className)} {...props}>
    <AvatarImage alt="" className="mt-0 mb-0" src={src} />
    <AvatarFallback>{name?.slice(0, 2) || "ME"}</AvatarFallback>
  </Avatar>
);
