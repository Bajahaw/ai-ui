"use client";

import {cn} from "@/lib/utils";
import {ComponentProps} from "react";

export interface WelcomeProps extends ComponentProps<"div"> {}

export const Welcome = ({ className, ...props }: WelcomeProps) => {
  return (
    <div
      className={cn("flex-1 flex items-center justify-center p-8", className)}
      {...props}
    >
      <div className="text-center">
        <h1 className="text-4xl font-semibold text-foreground">
          How can I help you?
        </h1>
      </div>
    </div>
  );
};
