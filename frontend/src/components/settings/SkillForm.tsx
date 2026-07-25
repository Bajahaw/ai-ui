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
import { SkillRequest, SkillResponse } from "@/lib/api/types";
import { getSkill } from "@/lib/api/skills";

interface SkillFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: SkillRequest) => Promise<void>;
  title: string;
  submitLabel: string;
  skill?: SkillResponse | null;
  /** View-only mode for built-in skills (no save / upload). */
  readOnly?: boolean;
}

export const SkillForm = ({
  open,
  onOpenChange,
  onSubmit,
  title,
  submitLabel,
  skill,
  readOnly = false,
}: SkillFormProps) => {
  const [formData, setFormData] = useState<SkillRequest>({
    id: "",
    name: "",
    description: "",
    content: "",
  });
  const [fileName, setFileName] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // When opening for edit, fetch the full skill content; when opening for
  // create, reset the form.
  useEffect(() => {
    if (!open) {
      setFormData({ id: "", name: "", description: "", content: "" });
      setFileName(null);
      setError(null);
      return;
    }
    if (skill) {
      setIsLoadingContent(true);
      setError(null);
      getSkill(skill.id)
        .then((detail) => {
          setFormData({
            id: detail.id,
            name: detail.name,
            description: detail.description,
            content: detail.content,
          });
        })
        .catch(() => {
          setError("Failed to load skill content");
          setFormData({
            id: skill.id,
            name: skill.name,
            description: skill.description,
            content: "",
          });
        })
        .finally(() => setIsLoadingContent(false));
    } else {
      setFormData({ id: "", name: "", description: "", content: "" });
      setFileName(null);
      setError(null);
    }
  }, [open, skill]);

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
    if (readOnly) {
      onOpenChange(false);
      return;
    }
    setError(null);

    if (!formData.name.trim()) {
      setError("Name is required");
      return;
    }

    if (!formData.content.trim()) {
      setError("Content cannot be empty");
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

  const fieldsDisabled = isSubmitting || isLoadingContent || readOnly;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px] p-0 flex flex-col rounded-xl">
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <DialogHeader className="px-6 pt-6 pb-2 flex-shrink-0">
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>

          {error && (
            <div className="mx-6 mb-2 flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md flex-shrink-0">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="px-6 space-y-4 flex-shrink-0">
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
                disabled={fieldsDisabled}
                required
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="skill-description">Description</Label>
              <Textarea
                id="skill-description"
                placeholder="Short description of what this skill teaches (shown to the model)..."
                value={formData.description}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    description: e.target.value,
                  }))
                }
                className="min-h-[60px] text-sm resize-none !bg-secondary/50 rounded-xl border-border/80 focus-visible:border-border"
                disabled={fieldsDisabled}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="skill-content">Content</Label>
            </div>
          </div>

          <div className="px-6 pb-4">
            {isLoadingContent ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground py-4">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>Loading content...</span>
              </div>
            ) : (
              <Textarea
                id="skill-content"
                placeholder="Paste or write the skill markdown here..."
                value={formData.content}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    content: e.target.value,
                  }))
                }
                className="h-40 text-sm font-mono resize-y overflow-y-auto !bg-secondary/50 rounded-xl border-border/80 focus-visible:border-border"
                disabled={fieldsDisabled}
              />
            )}
          </div>

          <DialogFooter className="px-6 pb-6 pt-2 flex-shrink-0 gap-2">
            <div className="flex-1" />
            {!readOnly && (
              <>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={fieldsDisabled}
                  className="gap-2"
                >
                  <Upload className="h-4 w-4" />
                  {fileName ? "Replace" : "Upload"}
                </Button>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".md,text/markdown"
                  onChange={handleFileChange}
                  disabled={fieldsDisabled}
                  className="hidden"
                />
                <Button
                  type="submit"
                  disabled={isSubmitting || isLoadingContent}
                >
                  {isSubmitting && (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  )}
                  {submitLabel}
                </Button>
              </>
            )}
            {readOnly && (
              <Button type="button" onClick={() => onOpenChange(false)}>
                Close
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
