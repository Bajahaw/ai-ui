import React, { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogTrigger,
} from "@/components/ui/dialog.tsx";
import { Button } from "@/components/ui/button.tsx";
import { UserRoundIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useAuth } from "@/hooks/useAuth.tsx";
import { cn } from "@/lib/utils.ts";

interface ProfilePickerDialogProps {
  children?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/**
 * Passwordless profile picker (profiles auth mode). Lists local profiles,
 * switches on tap, creates and deletes. Used in place of the login dialog
 * and from the account menu for switching.
 */
export const ProfilePickerDialog: React.FC<ProfilePickerDialogProps> = ({
  children,
  open,
  onOpenChange,
}) => {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const {
    profiles,
    activeProfile,
    selectProfile,
    createProfile,
    deleteProfile,
    isLoading,
    error,
    clearError,
  } = useAuth();

  const isControlled = open !== undefined && onOpenChange !== undefined;
  const dialogOpen = isControlled ? open : isDialogOpen;
  const setDialogOpen = isControlled ? onOpenChange : setIsDialogOpen;

  const handleOpenChange = (next: boolean) => {
    setDialogOpen(next);
    if (!next) {
      setNewName("");
      setConfirmDelete(null);
      clearError();
    }
  };

  const handleSelect = async (username: string) => {
    if (username === activeProfile) {
      setDialogOpen(false);
      return;
    }
    try {
      await selectProfile(username);
      setDialogOpen(false);
    } catch (err) {
      console.error("Profile switch failed:", err);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;
    try {
      await createProfile(name);
      setNewName("");
      setDialogOpen(false);
    } catch (err) {
      console.error("Profile creation failed:", err);
    }
  };

  const handleDelete = async (username: string) => {
    if (confirmDelete !== username) {
      setConfirmDelete(username);
      return;
    }
    try {
      await deleteProfile(username);
      setConfirmDelete(null);
    } catch (err) {
      console.error("Profile deletion failed:", err);
    }
  };

  return (
    <Dialog open={dialogOpen} onOpenChange={handleOpenChange}>
      {children ? <DialogTrigger asChild>{children}</DialogTrigger> : null}
      <DialogContent className="w-[min(92vw,28rem)] border-none bg-transparent p-0 shadow-none sm:rounded-2xl">
        <div className="flex flex-col items-center justify-center bg-background gap-6 p-6 sm:p-8 rounded-2xl border shadow-2xl">
          <div className="text-center space-y-2">
            <div className="flex items-center justify-center gap-3">
              <span className="flex size-9 items-center justify-center rounded-full bg-primary text-primary-foreground">
                <UserRoundIcon className="size-5" />
              </span>
              <h1 className="text-2xl font-bold">Who's chatting?</h1>
            </div>
            <p className="text-muted-foreground">
              Pick a profile — no sign-in needed. Each profile keeps its own
              chats, files and settings.
            </p>
          </div>

          <div className="w-full space-y-2">
            {profiles.map((username) => (
              <div
                key={username}
                className={cn(
                  "flex items-center gap-2 rounded-xl border p-2 pl-3 transition-colors",
                  username === activeProfile
                    ? "border-primary bg-primary/5"
                    : "border-input hover:border-primary/50",
                )}
              >
                <button
                  type="button"
                  onClick={() => void handleSelect(username)}
                  disabled={isLoading}
                  className="flex min-w-0 flex-1 items-center gap-3 text-left"
                >
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold">
                    {username.slice(0, 1).toUpperCase()}
                  </span>
                  <span className="truncate font-medium">{username}</span>
                  {username === activeProfile && (
                    <span className="shrink-0 text-xs text-muted-foreground">
                      current
                    </span>
                  )}
                </button>
                <button
                  type="button"
                  title={
                    confirmDelete === username
                      ? "Tap again to delete with all its data"
                      : `Delete ${username}`
                  }
                  onClick={() => void handleDelete(username)}
                  disabled={isLoading}
                  className={cn(
                    "shrink-0 rounded-lg p-2 transition-colors",
                    confirmDelete === username
                      ? "bg-destructive text-destructive-foreground"
                      : "text-muted-foreground hover:bg-secondary hover:text-destructive",
                  )}
                >
                  <Trash2Icon className="size-4" />
                </button>
              </div>
            ))}
            {profiles.length === 0 && (
              <p className="text-center text-sm text-muted-foreground">
                No profiles yet — create one below.
              </p>
            )}
          </div>

          <form onSubmit={handleCreate} className="flex w-full gap-2">
            <input
              type="text"
              placeholder="New profile name"
              value={newName}
              onChange={(e) => {
                setNewName(e.target.value);
                if (error) clearError();
              }}
              maxLength={32}
              disabled={isLoading}
              className="min-w-0 flex-1 rounded-xl border border-input bg-background px-4 py-2.5 text-foreground placeholder:text-muted-foreground transition-all focus:border-primary focus:outline-none focus:ring-[0.5px] focus:ring-primary/40"
            />
            <Button type="submit" disabled={isLoading || !newName.trim()}>
              <PlusIcon className="size-4" />
              Add
            </Button>
          </form>

          {error && (
            <p className="w-full text-sm text-destructive font-medium px-1">
              {error}
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};
