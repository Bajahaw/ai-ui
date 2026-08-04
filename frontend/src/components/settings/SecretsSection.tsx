import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "../ui/card";
import { Check, Copy, Edit, KeyRound, Plus, Trash2 } from "lucide-react";
import { SecretForm, secretRef } from "./SecretForm";
import { SecretRequest, SecretResponse } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";

export const SecretsSection = () => {
  const { data, addSecret, updateSecret, deleteSecret } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editing, setEditing] = useState<SecretResponse | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleAdd = async (secretData: SecretRequest) => {
    await addSecret(secretData);
    setShowAddForm(false);
  };

  const handleEdit = async (secretData: SecretRequest) => {
    await updateSecret(secretData);
    setEditing(null);
  };

  const handleDelete = async (id: string, name: string) => {
    if (confirm(`Delete secret "${name}"? This cannot be undone.`)) {
      await deleteSecret(id);
    }
  };

  const copySecretRef = async (id: string, name: string) => {
    try {
      await navigator.clipboard.writeText(secretRef(name));
      setCopiedId(id);
      setTimeout(() => setCopiedId((cur) => (cur === id ? null : cur)), 1500);
    } catch {
      // ignore
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium flex items-center gap-2">
          <KeyRound className="h-5 w-5" />
          Secrets
        </h3>
        <Button
          onClick={() => setShowAddForm(true)}
          variant="outline"
          size="sm"
        >
          <Plus className="h-4 w-4" />
          <span className="hidden sm:inline">Add Secret</span>
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        Store tokens for tools like{" "}
        <code className="text-xs bg-muted px-1 py-0.5 rounded">
          http_request
        </code>
        . Tell the AI the name when needed; it uses{" "}
        <code className="text-xs bg-muted px-1 py-0.5 rounded">
          $secrets.NAME$
        </code>
        .
      </p>

      {data.secrets.length === 0 ? (
        <Card className="p-6 text-center bg-transparent border-dashed">
          <div className="space-y-2">
            <p className="text-muted-foreground">No secrets configured</p>
            <Button
              onClick={() => setShowAddForm(true)}
              variant="outline"
              size="sm"
            >
              <Plus className="h-4 w-4" />
              Add Your First Secret
            </Button>
          </div>
        </Card>
      ) : (
        <div className="space-y-3">
          {data.secrets.map((secret) => (
            <Card
              key={secret.id}
              className="p-4 flex items-center justify-between gap-3 bg-transparent"
            >
              <div className="min-w-0">
                <p className="font-mono text-sm font-medium truncate">
                  {secret.name}
                </p>
                <button
                  type="button"
                  onClick={() => copySecretRef(secret.id, secret.name)}
                  className="inline-flex items-center gap-1 max-w-full text-xs text-muted-foreground font-mono hover:text-foreground"
                  title="Copy for chat"
                >
                  <span className="truncate">{secretRef(secret.name)}</span>
                  {copiedId === secret.id ? (
                    <Check className="size-3 shrink-0 text-green-600" />
                  ) : (
                    <Copy className="size-3 shrink-0" />
                  )}
                </button>
              </div>
              <div className="flex items-center gap-1 flex-shrink-0">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setEditing(secret)}
                  aria-label={`Edit ${secret.name}`}
                >
                  <Edit className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(secret.id, secret.name)}
                  aria-label={`Delete ${secret.name}`}
                >
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <SecretForm
        open={showAddForm}
        onOpenChange={setShowAddForm}
        onSubmit={handleAdd}
        title="Add Secret"
        submitLabel="Save"
      />
      <SecretForm
        open={!!editing}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        onSubmit={handleEdit}
        title="Edit Secret"
        submitLabel="Update"
        secret={editing}
      />
    </div>
  );
};
