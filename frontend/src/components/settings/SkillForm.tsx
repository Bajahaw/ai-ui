import { useState, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { AlertCircle, Loader2, Upload } from "lucide-react";
import { SkillRequest } from "@/lib/api/types";

interface SkillFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: SkillRequest) => Promise<void>;
  title: string;
  submitLabel: string;
}

export const SkillForm = ({
  open,
  onOpenChange,
  onSubmit,
  title,
  submitLabel,
}: SkillFormProps) => {
  const [formData, setFormData] = useState<SkillRequest>({
    name: "",
    description: "",
    content: "",
  });
  const [fileName, setFileName] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) {
      setFormData({ name: "", description: "", content: "" });
      setFileName(null);
      setError(null);
    }
  }, [open]);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith(".md") && file.type !== "text/markdown") {
      setError("Please select a markdown (.md) file");
      return;
    }

    setError(null);
    setFileName(file.name);

    const text = await file.text();
    setFormData((prev) => ({ ...prev, content: text }));

    // Auto-fill name from filename if empty
    if (!formData.name.trim()) {
      const baseName = file.name.replace(/\.md$/i, "");
      setFormData((prev) => ({ ...prev, name: baseName }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!formData.name.trim()) {
      setError("Name is required");
      return;
    }

    if (!formData.content.trim()) {
      setError("Please upload a markdown file");
      return;
    }

    setIsSubmitting(true);
    try {
      await onSubmit(formData);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save skill");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    setFormData({ name: "", description: "", content: "" });
    setFileName(null);
    setError(null);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px] p-6 rounded-xl">
        <DialogHeader className="pb-2">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="skill-file">Markdown File</Label>
            <input
              ref={fileInputRef}
              id="skill-file"
              type="file"
              accept=".md,text/markdown"
              onChange={handleFileChange}
              disabled={isSubmitting}
              className="hidden"
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => fileInputRef.current?.click()}
              disabled={isSubmitting}
              className="w-full gap-2"
            >
              <Upload className="h-4 w-4" />
              {fileName ? fileName : "Choose .md file"}
            </Button>
            {fileName && (
              <p className="text-xs text-muted-foreground">
                {formData.content.length.toLocaleString()} characters loaded
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="skill-name">Name</Label>
            <Input
              id="skill-name"
              type="text"
              placeholder="react-doctor"
              value={formData.name}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, name: e.target.value }))
              }
              disabled={isSubmitting}
              required
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="skill-description">Description</Label>
            <Textarea
              id="skill-description"
              placeholder="Short description of what this skill teaches..."
              value={formData.description}
              onChange={(e) =>
                setFormData((prev) => ({
                  ...prev,
                  description: e.target.value,
                }))
              }
              className="min-h-[60px] text-sm resize-none"
              disabled={isSubmitting}
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={handleCancel}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
