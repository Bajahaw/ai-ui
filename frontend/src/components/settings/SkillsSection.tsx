import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "../ui/card";
import { BookOpen, Edit, Plus, Trash2 } from "lucide-react";
import { SkillForm } from "./SkillForm";
import { SkillRequest, SkillResponse } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";

export const SkillsSection = () => {
  const { data, addSkill, updateSkill, deleteSkill } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingSkill, setEditingSkill] = useState<SkillResponse | null>(null);

  const handleAddSkill = async (skillData: SkillRequest) => {
    await addSkill(skillData);
    setShowAddForm(false);
  };

  const handleEditSkill = async (skillData: SkillRequest) => {
    await updateSkill(skillData);
    setEditingSkill(null);
  };

  const handleDeleteSkill = async (id: string, name: string) => {
    if (confirm(`Are you sure you want to delete the skill "${name}"?`)) {
      await deleteSkill(id);
    }
  };

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
        Skills are markdown documents that teach the AI specialized workflows or
        styles. The AI can list and read them via built-in tools during chat.
      </p>

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
        <div className="space-y-4 overflow-hidden">
          {data.skills.map((skill) => (
            <Card
              key={skill.id}
              className="p-4 bg-transparent overflow-hidden"
            >
              <div className="space-y-2">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <h4 className="truncate max-w-[75px] sm:max-w-[300px]">
                      {skill.name}
                    </h4>
                  </div>
                  <div className="flex items-center gap-1 flex-shrink-0">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setEditingSkill(skill)}
                      title="Edit skill"
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDeleteSkill(skill.id, skill.name)}
                      className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                      title="Delete skill"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
                {skill.description && (
                  <p className="text-sm text-muted-foreground line-clamp-2">
                    {skill.description}
                  </p>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Add Skill Form */}
      <SkillForm
        open={showAddForm}
        onOpenChange={setShowAddForm}
        onSubmit={handleAddSkill}
        title="Add Skill"
        submitLabel="Add Skill"
      />

      {/* Edit Skill Form */}
      <SkillForm
        open={!!editingSkill}
        onOpenChange={(open) => !open && setEditingSkill(null)}
        onSubmit={handleEditSkill}
        skill={editingSkill}
        title="Edit Skill"
        submitLabel="Save Changes"
      />
    </div>
  );
};
