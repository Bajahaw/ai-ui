import { SecretListResponse, SecretRequest, SecretResponse } from "./types";
import { getHeaders } from "./headers";

/** Placeholder the backend substitutes with the secret value. */
export const secretRef = (name: string) => `$secrets.${name}$`;

export const getSecrets = async (): Promise<SecretResponse[]> => {
  const response = await fetch("/api/secrets/all", {
    method: "GET",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch secrets: ${response.statusText}`);
  }

  const data: SecretListResponse = await response.json();
  return data.secrets;
};

export const saveSecret = async (
  secretData: SecretRequest,
): Promise<SecretResponse> => {
  const response = await fetch("/api/secrets/save", {
    method: "POST",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(secretData),
    credentials: "include",
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Failed to save secret: ${response.statusText}`);
  }

  return response.json();
};

export const deleteSecret = async (id: string): Promise<void> => {
  const response = await fetch(`/api/secrets/${encodeURIComponent(id)}`, {
    method: "DELETE",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to delete secret: ${response.statusText}`);
  }
};
