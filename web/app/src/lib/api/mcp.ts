import { apiFetch } from "./client";

export interface MCPToken {
  id: number;
  name: string;
  last_used_at?: string;
  created_at: string;
}

export interface CreatedMCPToken extends MCPToken {
  token: string;
}

export function listMCPTokens(): Promise<MCPToken[]> {
  return apiFetch<MCPToken[]>("/mcp/tokens");
}

export function createMCPToken(name: string): Promise<CreatedMCPToken> {
  return apiFetch<CreatedMCPToken>("/mcp/tokens", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function deleteMCPToken(id: number): Promise<void> {
  return apiFetch<void>(`/mcp/tokens/${id}`, { method: "DELETE" });
}
