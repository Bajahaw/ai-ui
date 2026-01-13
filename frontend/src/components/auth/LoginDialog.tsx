import React, { useState } from 'react';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog.tsx';
import { Button } from '@/components/ui/button.tsx';
import { Input } from '@/components/ui/input.tsx';
import { Label } from '@/components/ui/label.tsx';
import { LogInIcon, EyeIcon, EyeOffIcon } from 'lucide-react';
import { useAuth } from '@/hooks/useAuth.tsx';

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
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const { login, isLoading, error, clearError } = useAuth();

    const isControlled = open !== undefined && onOpenChange !== undefined;
    const dialogOpen = isControlled ? open : isDialogOpen;
    const setDialogOpen = isControlled ? onOpenChange : setIsDialogOpen;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!username.trim() || !password.trim()) return;

        try {
            await login(username.trim(), password.trim());
            setUsername('');
            setPassword('');
            setDialogOpen(false);
        } catch (err) {
            // Error is handled by the auth context
            console.error('Login failed:', err);
        }
    };

    const handleOpenChange = (newOpen: boolean) => {
        setDialogOpen(newOpen);
        if (!newOpen) {
            // Clear form and errors when dialog closes
            setUsername('');
            setPassword('');
            clearError();
        }
    };

    const handleUsernameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value);
        if (error) {
            clearError();
        }
    };

    const handlePasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setPassword(e.target.value);
        if (error) {
            clearError();
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
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Login</DialogTitle>
                    <DialogDescription>
                        Enter your credentials to access the chat interface.
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="username">Username</Label>
                        <Input
                            id="username"
                            type="text"
                            placeholder="Username"
                            value={username}
                            onChange={handleUsernameChange}
                            className={error ? 'border-destructive focus-visible:ring-destructive' : ''}
                            disabled={isLoading}
                            autoComplete="username"
                        />
                    </div>
                    <div className="space-y-2">
                        <Label htmlFor="password">Password</Label>
                        <div className="relative">
                            <Input
                                id="password"
                                type={showPassword ? 'text' : 'password'}
                                placeholder="Password"
                                value={password}
                                onChange={handlePasswordChange}
                                className={error ? 'border-destructive focus-visible:ring-destructive' : ''}
                                disabled={isLoading}
                                autoComplete="current-password"
                            />
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="absolute right-0 top-0 h-full px-3 py-2 hover:bg-transparent"
                                onClick={() => setShowPassword(!showPassword)}
                                disabled={isLoading}
                            >
                                {showPassword ? (
                                    <EyeOffIcon className="size-4" />
                                ) : (
                                    <EyeIcon className="size-4" />
                                )}
                            </Button>
                        </div>
                        {error && (
                            <p className="text-sm text-destructive">{error}</p>
                        )}
                    </div>
                    <div className="flex justify-end gap-2">
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => handleOpenChange(false)}
                            disabled={isLoading}
                        >
                            Cancel
                        </Button>
                        <Button type="submit" disabled={!username.trim() || !password.trim() || isLoading}>
                            {isLoading ? (
                                <>
                                    <div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent mr-2" />
                                    Logging in...
                                </>
                            ) : (
                                <>
                                    <LogInIcon className="size-4 mr-2" />
                                    Login
                                </>
                            )}
                        </Button>
                    </div>
                </form>
            </DialogContent>
        </Dialog>
    );
};
