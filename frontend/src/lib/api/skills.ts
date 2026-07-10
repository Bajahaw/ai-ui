import { SkillListResponse, SkillRequest, SkillResponse } from "./types";
import { getHeaders } from "./headers";

// Get all skills (list view, no content)
export const getSkills = async (): Promise<SkillResponse[]> => {
  const response = await fetch("/api/skills/all", {
    method: "GET",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch skills: ${response.statusText}`);
  }

  const data: SkillListResponse = await response.json();
  return data.skills;
};

// Save (add/replace) a skill from uploaded markdown content
export const saveSkill = async (
  skillData: SkillRequest,
): Promise<SkillResponse> => {
  const response = await fetch("/api/skills/save", {
    method: "POST",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(skillData),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to save skill: ${response.statusText}`);
  }

  return response.json();
};

// Delete a skill
export const deleteSkill = async (id: string): Promise<void> => {
  const response = await fetch(`/api/skills/${id}`, {
    method: "DELETE",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to delete skill: ${response.statusText}`);
  }
};
