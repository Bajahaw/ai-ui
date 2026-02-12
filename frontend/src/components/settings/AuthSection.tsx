import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Lock, User } from "lucide-react";
import { authAPI } from "@/lib/api/auth";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";

export const AuthSection = () => {
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [isSaving, setIsSaving] = useState(false);
    const [message, setMessage] = useState<{ text: string, type: "error" | "success" } | null>(null);
    const [open, setOpen] = useState(false);

    const handleSave = async () => {
        setMessage(null);
        if (password !== confirmPassword) {
            setMessage({ text: "Passwords do not match", type: "error" });
            return;
        }

        if (password.length < 8) {
            setMessage({ text: "Password must be at least 8 characters", type: "error" });
            return;
        }

        setIsSaving(true);
        try {
            await authAPI.changePassword(password);
            setMessage({ text: "Password changed successfully", type: "success" });
            setPassword("");
            setConfirmPassword("");
            setTimeout(() => {
                setOpen(false);
                setMessage(null);
            }, 1000);
        } catch (error) {
            setMessage({ text: error instanceof Error ? error.message : "Failed to change password", type: "error" });
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <div className="space-y-8 max-w-2xl">
            <h3 className="text-lg font-medium flex items-center gap-2">
                <User className="h-5 w-5" />
                Account Settings
            </h3>

            <div className="space-y-4">
                <div className="flex justify-between items-center pb-2">
                    <div className="space-y-1">
                        <Label className="text-base">Password</Label>
                        <p className="text-sm text-muted-foreground">
                            Change your account password
                        </p>
                    </div>
                    <Dialog open={open} onOpenChange={setOpen}>
                        <DialogTrigger asChild>
                            <Button variant="outline" size="sm">
                                <Lock className="mr-2 h-4 w-4" />
                                Change Password
                            </Button>
                        </DialogTrigger>
                        <DialogContent className="sm:max-w-[425px]">
                            <DialogHeader>
                                <DialogTitle>Change Password</DialogTitle>
                            </DialogHeader>
                            <div className="space-y-4 pt-4">
                                <div className="space-y-2">
                                    <Label htmlFor="password">New Password</Label>
                                    <Input
                                        id="password"
                                        type="password"
                                        value={password}
                                        onChange={(e) => setPassword(e.target.value)}
                                        placeholder="Enter new password"
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="confirm-password">Confirm Password</Label>
                                    <Input
                                        id="confirm-password"
                                        type="password"
                                        value={confirmPassword}
                                        onChange={(e) => setConfirmPassword(e.target.value)}
                                        placeholder="Confirm new password"
                                    />
                                </div>

                                {message && (
                                    <div className={`text-sm ${message.type === "error" ? "text-red-500" : "text-green-500"}`}>
                                        {message.text}
                                    </div>
                                )}

                                <div className="flex justify-end pt-2">
                                    <Button
                                        onClick={handleSave}
                                        disabled={isSaving || !password || !confirmPassword}
                                    >
                                        {isSaving ? "Saving..." : "Change Password"}
                                    </Button>
                                </div>
                            </div>
                        </DialogContent>
                    </Dialog>
                </div>
            </div>
        </div>
    );
};
