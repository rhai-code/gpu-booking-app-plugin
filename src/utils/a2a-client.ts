import { A2AAgentCard, A2AEvent } from './a2a-types';

const PLUGIN_NAME = 'gpu-booking-plugin';
const AGENT_PROXY = `/api/proxy/plugin/${PLUGIN_NAME}/agent`;

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)csrf-token=([^;]*)/);
  return match ? match[1] : '';
}

function generateId(): string {
  const arr = new Uint8Array(16);
  window.crypto.getRandomValues(arr);
  const hex = Array.from(arr, (b) => b.toString(16).padStart(2, '0')).join('');
  return [
    hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16),
    hex.slice(16, 20), hex.slice(20, 32),
  ].join('-');
}

export async function fetchAgentCard(): Promise<A2AAgentCard | null> {
  try {
    const res = await fetch(`${AGENT_PROXY}/.well-known/agent.json`, {
      headers: { 'X-CSRFToken': getCSRFToken() },
    });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export async function sendMessage(
  text: string,
  contextId?: string,
  taskId?: string,
): Promise<A2AEvent | null> {
  const body = {
    jsonrpc: '2.0',
    id: 1,
    method: 'message/send',
    params: {
      message: {
        role: 'user',
        parts: [{ kind: 'text', text }],
        messageId: generateId(),
        ...(contextId && { contextId }),
        ...(taskId && { taskId }),
      },
      ...(contextId && { configuration: { blocking: true } }),
    },
  };

  try {
    const res = await fetch(`${AGENT_PROXY}/`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRFToken': getCSRFToken(),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) return null;
    const data = await res.json();
    return data.result ?? null;
  } catch {
    return null;
  }
}

async function tryStreamMessage(
  body: Record<string, unknown>,
  onEvent: (event: A2AEvent) => void,
): Promise<boolean> {
  let res: Response;
  try {
    res = await fetch(`${AGENT_PROXY}/`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
        'X-CSRFToken': getCSRFToken(),
      },
      body: JSON.stringify({ ...body, method: 'message/stream' }),
    });
  } catch {
    return false;
  }

  if (!res.ok) return false;

  const contentType = res.headers.get('content-type') || '';

  if (!contentType.includes('text/event-stream')) {
    try {
      const data = await res.json();
      if (data.error) return false;
      const event = data.result ?? data;
      if (event.kind) onEvent(event as A2AEvent);
      return true;
    } catch {
      return false;
    }
  }

  const reader = res.body?.getReader();
  if (!reader) return false;

  const decoder = new TextDecoder();
  let buffer = '';

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed.startsWith('data:')) continue;

      const jsonStr = trimmed.slice(5).trim();
      if (!jsonStr) continue;

      try {
        const parsed = JSON.parse(jsonStr);
        const event = parsed.result ?? parsed;
        if (event.kind) onEvent(event as A2AEvent);
      } catch {
        // partial JSON chunk
      }
    }
  }

  return true;
}

export async function streamMessage(
  text: string,
  onEvent: (event: A2AEvent) => void,
  onDone: () => void,
  onError: (err: string) => void,
  contextId?: string,
  taskId?: string,
): Promise<void> {
  const messageId = generateId();
  const body = {
    jsonrpc: '2.0',
    id: 1,
    method: 'message/send',
    params: {
      message: {
        role: 'user',
        parts: [{ kind: 'text', text }],
        messageId,
        ...(contextId && { contextId }),
        ...(taskId && { taskId }),
      },
    },
  };

  const streamed = await tryStreamMessage(body, onEvent).catch(() => false);

  if (!streamed) {
    let res: Response;
    try {
      res = await fetch(`${AGENT_PROXY}/`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRFToken': getCSRFToken(),
        },
        body: JSON.stringify(body),
      });
    } catch (err) {
      onError(`Connection failed: ${err}`);
      return;
    }

    if (!res.ok) {
      onError(`Agent returned HTTP ${res.status}`);
      return;
    }

    try {
      const data = await res.json();
      if (data.error) {
        onError(data.error.message || JSON.stringify(data.error));
        return;
      }
      const event = data.result ?? data;
      if (event.kind) onEvent(event as A2AEvent);
    } catch {
      onError('Failed to parse agent response');
      return;
    }
  }

  onDone();
}
