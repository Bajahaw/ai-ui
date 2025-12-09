import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { authAPI } from '@/lib/api/auth.ts';

interface AuthContextType {
    isAuthenticated: boolean;
    isRegistered: boolean;
    isLoading: boolean;
    login: (token: string) => Promise<void>;
    logout: () => Promise<void>;
    register: () => Promise<string>;
    error: string | null;
    clearError: () => void;
    registeredToken: string | null;
    clearRegisteredToken: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface AuthProviderProps {
    children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [isRegistered, setIsRegistered] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [registeredToken, setRegisteredToken] = useState<string | null>(null);

    // Check authentication status on mount
    useEffect(() => {
        const checkAuth = async () => {
            try {
                const status = await authAPI.getAuthStatus();
                setIsRegistered(status.registered);
                setIsAuthenticated(status.authenticated);
            } catch (err) {
                console.error('Error checking auth status:', err);
                setIsRegistered(false);
                setIsAuthenticated(false);
            } finally {
                setIsLoading(false);
            }
        };

        checkAuth();
    }, []);

    const register = async (): Promise<string> => {
        try {
            setError(null);
            setIsLoading(true);
            const token = await authAPI.register();
            setIsRegistered(true);
            setRegisteredToken(token);
            return token;
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Registration failed';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    };

    const clearRegisteredToken = () => {
        setRegisteredToken(null);
    };

    const login = async (token: string) => {
        try {
            setError(null);
            setIsLoading(true);
            await authAPI.login(token);
            // After login, re-check auth status
            const status = await authAPI.getAuthStatus();
            if (!status.authenticated) {
                throw new Error('Login succeeded but user is not authenticated, this is usually happen due to using secure cookie in a non-secure context. Please use HTTPS or access the app via localhost.');
            }
            setIsAuthenticated(true);
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Login failed';
            setError(errorMessage);
            setIsAuthenticated(false);
            throw err;
        } finally {
            setIsLoading(false);
        }
    };

    const logout = async () => {
        try {
            setError(null);
            setIsLoading(true);
            await authAPI.logout();
            setIsAuthenticated(false);
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Logout failed';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    };

    const clearError = () => {
        setError(null);
    };

    const value: AuthContextType = {
        isAuthenticated,
        isRegistered,
        isLoading,
        login,
        logout,
        register,
        error,
        clearError,
        registeredToken,
        clearRegisteredToken,
    };

    return (
        <AuthContext.Provider value={value}>
            {children}
        </AuthContext.Provider>
    );
};

export const useAuth = (): AuthContextType => {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
};
