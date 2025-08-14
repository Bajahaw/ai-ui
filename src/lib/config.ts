// Environment configuration for API base URLs
export interface ApiConfig {
  baseUrl: string;
  timeout: number;
  retries: number;
}

const getApiBaseUrl = (): string => {

  const isDevelopment = import.meta.env.DEV;
  if (isDevelopment) {
    return import.meta.env.VITE_API_BASE_URL || 'http://localhost:3000';
  }

  const productionBaseUrl = import.meta.env.VITE_API_BASE_URL;
  if (productionBaseUrl) {
    return productionBaseUrl;
  }

  return '';
};

export const apiConfig: ApiConfig = {
  baseUrl: getApiBaseUrl(),
  timeout: 30000, // 30 seconds
  retries: 3,
};

// Helper function to get full API URL
export const getApiUrl = (endpoint: string): string => {
  const baseUrl = apiConfig.baseUrl;
  const cleanEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;

  if (baseUrl) {
    return `${baseUrl.replace(/\/$/, '')}${cleanEndpoint}`;
  }

  return cleanEndpoint;
};

// Environment info for debugging
export const envInfo = {
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
  baseUrl: apiConfig.baseUrl,
  mode: import.meta.env.MODE,
};

// Log configuration in development
if (import.meta.env.DEV) {
  console.log('API Configuration:', {
    baseUrl: apiConfig.baseUrl,
    environment: import.meta.env.MODE,
  });
}
