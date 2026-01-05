import React, { useState } from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import "./globals.css";
import { ThemeProvider } from "./components/theme-provider.tsx";
import { AuthProvider, useAuth } from "./hooks/useAuth.tsx";
import { ModelsProvider } from "./hooks/useModelsContext.tsx";
import { SettingsDataProvider } from "./hooks/useSettingsData.tsx";

/**
 * RegistrationSuccess - Shows the generated token
 */
const RegistrationSuccess = ({ token, onContinue }: { token: string; onContinue: () => void }) => {
	const [copied, setCopied] = useState(false);

	const copyToClipboard = async () => {
		await navigator.clipboard.writeText(token);
		setCopied(true);
		setTimeout(() => setCopied(false), 2000);
	};

	return (
		<div className="flex flex-col items-center justify-center h-screen bg-background gap-6 p-4">
			<div className="text-center space-y-2">
				<div className="flex items-center justify-center gap-3">
					<svg width="36" height="36" viewBox="0 0 1191 1191" xmlns="http://www.w3.org/2000/svg">
						<circle cx="595.276" cy="614.849" r="499.517" className="fill-foreground" />
						<path d="M924.054,572.425c0,82.98 -73.269,158.521 -188.149,193.982l-341.54,105.426l-112.51,-235.507c-9.883,-20.687 -14.91,-42.231 -14.91,-63.901c0,-118.419 147.22,-214.56 328.554,-214.56c181.334,0 328.554,96.141 328.554,214.56Z" className="fill-background" />
					</svg>
					<h1 className="text-2xl font-bold">AI Chat</h1>
				</div>
				<p className="text-muted-foreground">Save your token</p>
			</div>

			<div className="w-full max-w-sm space-y-4">
				<div className="relative">
					<input
						type="text"
						value={token}
						readOnly
						className="w-full px-4 py-2.5 pr-10 rounded-xl border bg-background text-foreground font-mono text-sm select-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0 border-input focus:ring-primary/40 focus:border-primary"
					/>
					<button
						type="button"
						onClick={copyToClipboard}
						className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
						tabIndex={-1}
					>
						{copied ? (
							<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
								<polyline points="20 6 9 17 4 12" />
							</svg>
						) : (
							<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
								<rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
								<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
							</svg>
						)}
					</button>
				</div>

				<button
					onClick={onContinue}
					className="w-full px-6 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
				>
					Continue to Login
				</button>
			</div>
		</div>
	);
};

/**
 * LoginScreen - Inline login/registration form
 */
const LoginScreen = () => {
	const [token, setToken] = useState("");
	const [showToken, setShowToken] = useState(false);
	const { login, register, isRegistered, isLoading, error, clearError, registeredToken, clearRegisteredToken } = useAuth();

	const handleRegister = async () => {
		try {
			await register();
		} catch (err) {
			// Error is handled by auth context and displayed below
			console.error('Registration failed:', err);
		}
	};

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!token.trim() || isLoading) return;

		try {
			await login(token.trim());
		} catch (err) {
			// Error is handled by auth context and displayed below
		}
	};

	const handleTokenChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		setToken(e.target.value);
		if (error) {
			clearError();
		}
	};

	// Show generated token screen
	if (registeredToken) {
		return (
			<RegistrationSuccess
				token={registeredToken}
				onContinue={clearRegisteredToken}
			/>
		);
	}

	return (
		<div className="flex flex-col items-center justify-center h-screen bg-background gap-6 p-4">
			<div className="text-center space-y-2">
				<div className="flex items-center justify-center gap-3">
					{/* App Logo inline with title */}
					<svg width="36" height="36" viewBox="0 0 1191 1191" xmlns="http://www.w3.org/2000/svg">
						<circle cx="595.276" cy="614.849" r="499.517" className="fill-foreground" />
						<path d="M924.054,572.425c0,82.98 -73.269,158.521 -188.149,193.982l-341.54,105.426l-112.51,-235.507c-9.883,-20.687 -14.91,-42.231 -14.91,-63.901c0,-118.419 147.22,-214.56 328.554,-214.56c181.334,0 328.554,96.141 328.554,214.56Z" className="fill-background" />
					</svg>
					<h1 className="text-2xl font-bold">AI Chat</h1>
				</div>
				<p className="text-muted-foreground">
					{isRegistered
						? "Enter your authentication token to continue"
						: "Register to create your authentication token"}
				</p>
			</div>

			{!isRegistered ? (
				<div className="w-full max-w-sm space-y-4">
					<button
						onClick={handleRegister}
						disabled={isLoading}
						className="w-full px-6 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{isLoading ? "Registering..." : "Register"}
					</button>
					{error && (
						<p className="text-sm text-destructive text-center">{error}</p>
					)}
				</div>
			) : (
				<form onSubmit={handleSubmit} className="w-full max-w-sm space-y-4">
					<div className="space-y-2">
						<div className="relative">
							<input
								type={showToken ? "text" : "password"}
								placeholder="Enter your token..."
								value={token}
								onChange={handleTokenChange}
								className={`w-full px-4 py-2.5 pr-10 rounded-xl border bg-background text-foreground placeholder:text-muted-foreground transition-all focus:outline-none focus:ring-[0.5px] focus:ring-offset-0 ${error ? "border-destructive focus:ring-destructive" : "border-input focus:ring-primary/40 focus:border-primary"
									}`}
								disabled={isLoading}
								autoComplete="off"
								autoFocus
							/>
							<button
								type="button"
								onClick={() => setShowToken(!showToken)}
								className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
								tabIndex={-1}
							>
								{showToken ? (
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" /><line x1="1" y1="1" x2="23" y2="23" /></svg>
								) : (
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
								)}
							</button>
						</div>

						{error && (
							<p className="text-sm text-destructive">{error}</p>
						)}
					</div>

					<button
						type="submit"
						disabled={isLoading || !token.trim()}
						className="w-full px-6 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{isLoading ? "Logging in..." : "Login"}
					</button>
				</form>
			)}
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
