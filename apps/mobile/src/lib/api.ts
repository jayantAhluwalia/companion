import { loadSettings } from './settings';

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function request<T>(method: 'GET' | 'POST' | 'PATCH', path: string, body?: unknown): Promise<T> {
  const { apiUrl, apiToken } = await loadSettings();
  if (!apiUrl || !apiToken) throw new ApiError(0, 'Set the API URL and token first.');

  const res = await fetch(`${apiUrl}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${apiToken}`,
      'Content-Type': 'application/json',
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let json: unknown = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    // non-JSON error body; fall through with text
  }
  if (!res.ok) {
    const msg = (json as { error?: string } | null)?.error ?? text ?? res.statusText;
    throw new ApiError(res.status, `${res.status}: ${msg}`);
  }
  return json as T;
}

export const api = {
  health: () => request<{ ok: string }>('GET', '/healthz'),
  registerDevice: (expoPushToken: string) =>
    request<{ status: string }>('POST', '/v1/devices', { expo_push_token: expoPushToken }),
  pushTest: (title: string, body: string) =>
    request<{ tickets: Record<string, string> }>('POST', '/v1/push/test', { title, body }),
  spike: (rawContent: string) =>
    request<{ interpretation: Interpretation; call: CallInfo; tickets: Record<string, string> }>(
      'POST',
      '/v1/spike',
      { raw_content: rawContent },
    ),
};

export type Interpretation = {
  type: string;
  title: string;
  description: string;
  desired_outcome: string;
  next_steps: { label: string; estimated_minutes: number }[];
  confidence: number;
  clarification_needed: boolean;
  clarification_question: string;
  assumptions: string[];
};

export type CallInfo = {
  Model: string;
  PromptVersion: string;
  InputTokens: number;
  OutputTokens: number;
  Latency: number;
  StopReason: string;
};
