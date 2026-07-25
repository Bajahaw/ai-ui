import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "../ui/card";
import { BookOpen, Edit, Eye, Plus, Trash2 } from "lucide-react";
import { SkillForm } from "./SkillForm";
import { SkillRequest, SkillResponse } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";

export const SkillsSection = () => {
  const { data, addSkill, updateSkill, deleteSkill } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingSkill, setEditingSkill] = useState<SkillResponse | null>(null);
  const [viewOnly, setViewOnly] = useState(false);

  const builtinsEnabled =
    data.settings.enableBuiltinSkills !== "false";

  const handleAddSkill = async (skillData: SkillRequest) => {
    await addSkill(skillData);
    setShowAddForm(false);
  };

  const handleEditSkill = async (skillData: SkillRequest) => {
    await updateSkill(skillData);
    setEditingSkill(null);
    setViewOnly(false);
  };

  const handleDeleteSkill = async (id: string, name: string) => {
    if (id.startsWith("builtin:")) {
      return;
    }
    if (confirm(`Are you sure you want to delete the skill "${name}"?`)) {
      await deleteSkill(id);
    }
  };

  const openSkill = (skill: SkillResponse, readOnly: boolean) => {
    setViewOnly(readOnly);
    setEditingSkill(skill);
  };

  const userSkills = data.skills.filter((s) => !s.builtin);
  const builtinSkills = data.skills.filter((s) => s.builtin);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium flex items-center gap-2">
          <BookOpen className="h-5 w-5" />
          Skills
        </h3>
        <Button
          onClick={() => setShowAddForm(true)}
          variant="outline"
          size="sm"
        >
          <Plus className="h-4 w-4" />
          <span className="hidden sm:inline">Add Skill</span>
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        Skills are markdown documents that teach the AI specialized workflows.
        The model only sees skills listed as available (via{" "}
        <code className="text-xs">read_skill</code>). Built-in skills can be
        turned off for your account under{" "}
        <span className="font-medium text-foreground">General → Enable built-in skills</span>
        . Your skills always win over a built-in with the same name.
      </p>

      {!builtinsEnabled && builtinSkills.length > 0 && (
        <p className="text-sm text-amber-700 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-900 rounded-lg px-3 py-2">
          Built-in skills are disabled for your account. They stay listed below
          but are hidden from the model until you re-enable them in General
          settings.
        </p>
      )}

      {data.skills.length === 0 ? (
        <Card className="p-6 text-center bg-transparent border-dashed">
          <div className="space-y-2">
            <p className="text-muted-foreground">No skills configured</p>
            <Button
              onClick={() => setShowAddForm(true)}
              variant="outline"
              size="sm"
            >
              <Plus className="h-4 w-4" />
              Add Your First Skill
            </Button>
          </div>
        </Card>
      ) : (
        <div className="space-y-6 overflow-hidden">
          {userSkills.length > 0 && (
            <div className="space-y-3">
              <h4 className="text-sm font-medium text-muted-foreground">
                Your skills
              </h4>
              {userSkills.map((skill) => (
                <SkillCard
                  key={skill.id}
                  skill={skill}
                  onEdit={() => openSkill(skill, false)}
                  onDelete={() => handleDeleteSkill(skill.id, skill.name)}
                />
              ))}
            </div>
          )}

          {builtinSkills.length > 0 && (
            <div className="space-y-3">
              <h4 className="text-sm font-medium text-muted-foreground">
                Built-in skills
                {!builtinsEnabled && (
                  <span className="ml-2 text-xs font-normal">(disabled)</span>
                )}
              </h4>
              {builtinSkills.map((skill) => (
                <SkillCard
                  key={skill.id}
                  skill={skill}
                  inactive={!builtinsEnabled}
                  onView={() => openSkill(skill, true)}
                />
              ))}
            </div>
          )}
        </div>
      )}

      <SkillForm
        open={showAddForm}
        onOpenChange={setShowAddForm}
        onSubmit={handleAddSkill}
        title="Add Skill"
        submitLabel="Add Skill"
      />

      <SkillForm
        open={!!editingSkill}
        onOpenChange={(open) => {
          if (!open) {
            setEditingSkill(null);
            setViewOnly(false);
          }
        }}
        onSubmit={handleEditSkill}
        skill={editingSkill}
        title={viewOnly ? "View Built-in Skill" : "Edit Skill"}
        submitLabel="Save Changes"
        readOnly={viewOnly}
      />
    </div>
  );
};

function SkillCard({
  skill,
  inactive = false,
  onEdit,
  onView,
  onDelete,
}: {
  skill: SkillResponse;
  inactive?: boolean;
  onEdit?: () => void;
  onView?: () => void;
  onDelete?: () => void;
}) {
  return (
    <Card
      className={`p-4 bg-transparent overflow-hidden ${
        inactive ? "opacity-60" : ""
      }`}
    >
      <div className="space-y-2">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h4 className="truncate max-w-[75px] sm:max-w-[280px]">
                {skill.name}
              </h4>
              {skill.builtin && (
                <span className="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                  System
                </span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            {onView && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onView}
                title="View skill"
              >
                <Eye className="h-4 w-4" />
              </Button>
            )}
            {onEdit && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onEdit}
                title="Edit skill"
              >
                <Edit className="h-4 w-4" />
              </Button>
            )}
            {onDelete && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onDelete}
                className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                title="Delete skill"
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>
        {skill.description && (
          <p className="text-sm text-muted-foreground line-clamp-2">
            {skill.description}
          </p>
        )}
      </div>
    </Card>
  );
}
