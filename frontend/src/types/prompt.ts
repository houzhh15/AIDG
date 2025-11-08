/**
 * Prompt types for custom MCP prompts management
 */

export interface Prompt {
  prompt_id: string;
  name: string;
  description?: string;
  content: string;
  arguments?: PromptArgument[];
  scope: 'global' | 'project' | 'personal';
  visibility: 'public' | 'private';
  owner: string;
  project_id?: string;
  version: number;
  created_at: string; // ISO 8601 format
  updated_at: string; // ISO 8601 format
}

export interface PromptArgument {
  name: string;
  description?: string;
  required: boolean;
}

export type PromptScope = 'global' | 'project' | 'personal';
export type PromptVisibility = 'public' | 'private';
