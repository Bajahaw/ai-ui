import React, { useState } from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import "./globals.css";
import { ThemeProvider } from "./components/theme-provider.tsx";
import { AuthProvider, useAuth } from "./hooks/useAuth.tsx";
import { ModelsProvider } from "./hooks/useModelsContext.tsx";
import { SettingsDataProvider } from "./hooks/useSettingsData.tsx";
import { Collapsible, CollapsibleContent } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

/**
 * LoginScreen - Inline login/registration form
 */
const LoginScreen = () => {
    // Mode state: true = Login, false = Register
    const [isLoginMode, setIsLoginMode] = useState(true);
    
    // Form fields
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [confirmPassword, setConfirmPassword] = useState("");
	
    // UI toggles
    const [showPassword, setShowPassword] = useState(false);
	
    const { login, register, isLoading, error, clearError } = useAuth();
    
    // Local validation error
    const [validationError, setValidationError] = useState<string | null>(null);

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
		} catch (err) {
			// Error is handled by auth context and displayed below
		}
	};

    const toggleMode = () => {
        setIsLoginMode(!isLoginMode);
        setValidationError(null);
        clearError();
        // Keep username/password filled or clear them? Usually clear password.
        setPassword("");
        setConfirmPassword("");
    };

    // Generic change handlers clearing errors
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
    
    const handleConfirmPasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		setConfirmPassword(e.target.value);
		if (error) clearError();
        if (validationError) setValidationError(null);
	};

	return (
		<div className="flex flex-col items-center justify-center h-screen bg-background gap-6 p-4">
			<div className="text-center space-y-2 transition-all duration-300 ease-in-out">
				<div className="flex items-center justify-center gap-3">
					{/* App Logo inline with title */}
					<svg width="36" height="36" viewBox="0 0 1191 1191" xmlns="http://www.w3.org/2000/svg">
						<circle cx="595.276" cy="614.849" r="499.517" className="fill-foreground" />
						<path d="M924.054,572.425c0,82.98 -73.269,158.521 -188.149,193.982l-341.54,105.426l-112.51,-235.507c-9.883,-20.687 -14.91,-42.231 -14.91,-63.901c0,-118.419 147.22,-214.56 328.554,-214.56c181.334,0 328.554,96.141 328.554,214.56Z" className="fill-background" />
					</svg>
					<h1 className="text-2xl font-bold">AI Chat</h1>
				</div>
				<p className="text-muted-foreground animate-in fade-in slide-in-from-top-2 duration-300" key={isLoginMode ? 'login-text' : 'register-text'}>
					{isLoginMode
						? "Enter your credentials to continue"
						: "Create a new account"}
				</p>
			</div>

			<form onSubmit={handleSubmit} className="w-full max-w-sm space-y-4">
				<div className="space-y-4">
					<div className="space-y-2">
						<input
							type="text"
							placeholder="Username"
							value={username}
							onChange={handleUsernameChange}
							className={cn(
								"w-full px-4 py-2.5 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0",
								error || validationError ? "border-destructive focus:ring-destructive" : "border-input focus:ring-primary/40 focus:border-primary"
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
								error || validationError ? "border-destructive focus:ring-destructive" : "border-input focus:ring-primary/40 focus:border-primary"
							)}
							disabled={isLoading}
							autoComplete={isLoginMode ? "current-password" : "new-password"}
						/>
						<button
							type="button"
							onClick={() => setShowPassword(!showPassword)}
							className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
							tabIndex={-1}
						>
							{showPassword ? (
								<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" /><line x1="1" y1="1" x2="23" y2="23" /></svg>
							) : (
								<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
							)}
						</button>
					</div>

					<Collapsible open={!isLoginMode} className="w-full space-y-2">
						<CollapsibleContent className="overflow-hidden data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down">
							<div className="space-y-2 pt-1">
								<input
									type="password"
									placeholder="Confirm Password"
									value={confirmPassword}
									onChange={handleConfirmPasswordChange}
									className={cn(
										"w-full px-4 py-2.5 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0",
										error || validationError ? "border-destructive focus:ring-destructive" : "border-input focus:ring-primary/40 focus:border-primary"
									)}
									disabled={isLoading}
									autoComplete="new-password"
								/>
							</div>
						</CollapsibleContent>
					</Collapsible>

					<Collapsible open={!!(error || validationError)} className="w-full">
						<CollapsibleContent className="overflow-hidden data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down">
							<p className="text-sm text-destructive font-medium pt-1 px-1">
								{validationError || error}
							</p>
						</CollapsibleContent>
					</Collapsible>
				</div>

				<button
					type="submit"
					disabled={isLoading || !username.trim() || !password.trim() || (!isLoginMode && !confirmPassword.trim())}
					className="w-full px-6 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{isLoading
						? (isLoginMode ? "Logging in..." : "Registering...")
						: (isLoginMode ? "Login" : "Register")}
				</button>
                
                <div className="pt-2 text-center">
                    <button
                        type="button"
                        onClick={toggleMode}
                        className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
                        disabled={isLoading}
                    >
                        {isLoginMode ? "Don't have an account? Register" : "Already have an account? Login"}
                    </button>
                </div>
			</form>
		</div>
	);
};

/**
 * AuthGuard - Shows login screen when not authenticated, otherwise renders the app.
 * This prevents API calls from firing before authentication.
 */
const AuthGuard = ({ children }: { children: React.ReactNode }) => {
	const { isAuthenticated, isLoading } = useAuth();

	// While checking auth status, show loading
	if (isLoading) {
		return (
			<div className="flex items-center justify-center h-screen bg-background">
				<div className="text-muted-foreground">Checking authentication...</div>
			</div>
		);
	}

	// When not authenticated, show login screen instead of the main app
	// This prevents any API calls from being made
	if (!isAuthenticated) {
		return <LoginScreen />;
	}

	// Only render children (which trigger API calls) when authenticated
	return <>{children}</>;
};

const isDevelopment = (import.meta as any).env.DEV;

// Conditionally wrap with StrictMode - disable in dev to prevent duplicate messages
// AuthProvider must be outermost so auth state is available to guard components
const AppWrapper = isDevelopment ? (
	<AuthProvider>
		<ThemeProvider defaultTheme="dark" storageKey="ai-ui-theme">
			<AuthGuard>
				<ModelsProvider>
					<SettingsDataProvider>
						<App />
					</SettingsDataProvider>
				</ModelsProvider>
			</AuthGuard>
		</ThemeProvider>
	</AuthProvider>
) : (
	<React.StrictMode>
		<AuthProvider>
			<ThemeProvider defaultTheme="dark" storageKey="ai-ui-theme">
				<AuthGuard>
					<ModelsProvider>
						<SettingsDataProvider>
							<App />
						</SettingsDataProvider>
					</ModelsProvider>
				</AuthGuard>
			</ThemeProvider>
		</AuthProvider>
	</React.StrictMode>
);

ReactDOM.createRoot(document.getElementById("root")!).render(AppWrapper);
