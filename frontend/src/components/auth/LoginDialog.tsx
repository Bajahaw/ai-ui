import React, { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogTrigger,
} from "@/components/ui/dialog.tsx";
import { Button } from "@/components/ui/button.tsx";
import { LogInIcon } from "lucide-react";
import { useAuth } from "@/hooks/useAuth.tsx";
import { cn } from "@/lib/utils.ts";

interface LoginDialogProps {
  children?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export const LoginDialog: React.FC<LoginDialogProps> = ({
  children,
  open,
  onOpenChange,
}) => {
  const [isLoginMode, setIsLoginMode] = useState(true);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const { login, register, loginWithChatGPT, isLoading, error, clearError } =
    useAuth();
  const [validationError, setValidationError] = useState<string | null>(null);
  const [chatgptLoading, setChatgptLoading] = useState(false);

  const isControlled = open !== undefined && onOpenChange !== undefined;
  const dialogOpen = isControlled ? open : isDialogOpen;
  const setDialogOpen = isControlled ? onOpenChange : setIsDialogOpen;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    if (!username.trim() || !password.trim()) return;

    if (!isLoginMode && password !== confirmPassword) {
      setValidationError("Passwords do not match");
      return;
    }

    if (!isLoginMode && password.length < 8) {
      setValidationError("Password must be at least 8 characters");
      return;
    }

    try {
      if (isLoginMode) {
        await login(username.trim(), password.trim());
      } else {
        await register(username.trim(), password.trim());
      }
      setUsername("");
      setPassword("");
      setConfirmPassword("");
      setDialogOpen(false);
    } catch (err) {
      // Error is handled by the auth context
      console.error("Login failed:", err);
    }
  };

  const handleOpenChange = (newOpen: boolean) => {
    setDialogOpen(newOpen);
    if (!newOpen) {
      // Clear form and errors when dialog closes
      setIsLoginMode(true);
      setUsername("");
      setPassword("");
      setConfirmPassword("");
      setValidationError(null);
      clearError();
    }
  };

  const handleUsernameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUsername(e.target.value);
    if (error) clearError();
    if (validationError) setValidationError(null);
  };

  const handlePasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPassword(e.target.value);
    if (error) clearError();
    if (validationError) setValidationError(null);
  };

  const handleConfirmPasswordChange = (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    setConfirmPassword(e.target.value);
    if (error) clearError();
    if (validationError) setValidationError(null);
  };

  const toggleMode = () => {
    setIsLoginMode(!isLoginMode);
    setValidationError(null);
    clearError();
    setPassword("");
    setConfirmPassword("");
  };

  const handleChatGPTSignIn = async () => {
    setValidationError(null);
    clearError();
    setChatgptLoading(true);
    try {
      await loginWithChatGPT();
      setDialogOpen(false);
    } catch (err) {
      console.error("ChatGPT sign-in failed:", err);
    } finally {
      setChatgptLoading(false);
    }
  };

  return (
    <Dialog open={dialogOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        {children || (
          <Button variant="outline" className="w-full justify-start gap-2">
            <LogInIcon className="size-4" />
            Login
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="w-[min(92vw,28rem)] border-none bg-transparent p-0 shadow-none sm:rounded-2xl">
        <div className="flex flex-col items-center justify-center bg-background gap-6 p-6 sm:p-8 rounded-2xl border shadow-2xl">
          <div className="text-center space-y-2 transition-all duration-300 ease-in-out">
            <div className="flex items-center justify-center gap-3">
              <svg
                width="36"
                height="36"
                viewBox="0 0 1191 1191"
                xmlns="http://www.w3.org/2000/svg"
              >
                <circle
                  cx="595.276"
                  cy="614.849"
                  r="499.517"
                  className="fill-foreground"
                />
                <path
                  d="M924.054,572.425c0,82.98 -73.269,158.521 -188.149,193.982l-341.54,105.426l-112.51,-235.507c-9.883,-20.687 -14.91,-42.231 -14.91,-63.901c0,-118.419 147.22,-214.56 328.554,-214.56c181.334,0 328.554,96.141 328.554,214.56Z"
                  className="fill-background"
                />
              </svg>
              <h1 className="text-2xl font-bold">AI Chat</h1>
            </div>
            <p
              className="text-muted-foreground animate-in fade-in slide-in-from-top-2 duration-300"
              key={isLoginMode ? "login-text" : "register-text"}
            >
              {isLoginMode
                ? "Enter your credentials to continue"
                : "Create a new account"}
            </p>
          </div>

          <form onSubmit={handleSubmit} className="w-full space-y-4">
            <div className="flex flex-col gap-5">
              <div className="space-y-2">
                <input
                  type="text"
                  placeholder="Username"
                  value={username}
                  onChange={handleUsernameChange}
                  className={cn(
                    "w-full px-4 py-2.5 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0",
                    error || validationError
                      ? "border-destructive focus:ring-destructive"
                      : "border-input focus:ring-primary/40 focus:border-primary",
                  )}
                  disabled={isLoading}
                  autoComplete="username"
                  autoFocus
                />
              </div>
              <div className="relative">
                <input
                  type={showPassword ? "text" : "password"}
                  placeholder="Password"
                  value={password}
                  onChange={handlePasswordChange}
                  className={cn(
                    "w-full px-4 py-2.5 pr-10 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0",
                    error || validationError
                      ? "border-destructive focus:ring-destructive"
                      : "border-input focus:ring-primary/40 focus:border-primary",
                  )}
                  disabled={isLoading}
                  autoComplete={
                    isLoginMode ? "current-password" : "new-password"
                  }
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                  tabIndex={-1}
                >
                  {showPassword ? (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
                      <line x1="1" y1="1" x2="23" y2="23" />
                    </svg>
                  ) : (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                      <circle cx="12" cy="12" r="3" />
                    </svg>
                  )}
                </button>
              </div>

              <div
                className={cn(
                  "grid w-full transition-all !duration-200 ease-in-out",
                  !isLoginMode
                    ? "grid-rows-[1fr] opacity-100"
                    : "grid-rows-[0fr] opacity-0 -mt-4",
                )}
              >
                <div className="overflow-hidden">
                  <div className="space-y-2 pt-1">
                    <input
                      type="password"
                      placeholder="Confirm Password"
                      value={confirmPassword}
                      onChange={handleConfirmPasswordChange}
                      className={cn(
                        "w-full px-4 py-2.5 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0",
                        error || validationError
                          ? "border-destructive focus:ring-destructive"
                          : "border-input focus:ring-primary/40 focus:border-primary",
                      )}
                      disabled={isLoading}
                      autoComplete="new-password"
                    />
                  </div>
                </div>
              </div>

              <div
                className={cn(
                  "grid w-full transition-all !duration-200 ease-in-out",
                  error || validationError
                    ? "grid-rows-[1fr] opacity-100"
                    : "grid-rows-[0fr] opacity-0 -mt-4",
                )}
              >
                <div className="overflow-hidden">
                  <p className="text-sm text-destructive font-medium pt-1 px-1">
                    {validationError || error}
                  </p>
                </div>
              </div>
            </div>

            <button
              type="submit"
              disabled={
                isLoading ||
                chatgptLoading ||
                !username.trim() ||
                !password.trim() ||
                (!isLoginMode && !confirmPassword.trim())
              }
              className="w-full px-6 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoading
                ? isLoginMode
                  ? "Logging in..."
                  : "Registering..."
                : isLoginMode
                  ? "Login"
                  : "Register"}
            </button>

            <div className="relative py-1">
              <div className="absolute inset-0 flex items-center">
                <span className="w-full border-t border-border" />
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="bg-background px-2 text-muted-foreground">
                  or
                </span>
              </div>
            </div>

            <button
              type="button"
              onClick={handleChatGPTSignIn}
              disabled={isLoading || chatgptLoading}
              className="w-full px-6 py-2.5 rounded-lg border border-input bg-background hover:bg-muted/60 transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 font-medium"
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="currentColor"
                aria-hidden
              >
                <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.395-.682zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" />
              </svg>
              {chatgptLoading
                ? "Waiting for ChatGPT..."
                : "Continue with ChatGPT"}
            </button>
            <p className="text-[11px] text-center text-muted-foreground leading-snug">
              Signs you in and adds ChatGPT as a provider so you can start
              chatting right away.
            </p>

            <div className="pt-2 text-center">
              <button
                type="button"
                onClick={toggleMode}
                className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
                disabled={isLoading || chatgptLoading}
              >
                {isLoginMode
                  ? "Don't have an account? Register"
                  : "Already have an account? Login"}
              </button>
            </div>
          </form>
        </div>
      </DialogContent>
    </Dialog>
  );
};
