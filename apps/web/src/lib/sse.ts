// Helper para conexión SSE robusta usando fetch para soportar cabeceras de autenticación Bearer

import { camelizeResponse, getApiBaseUrl, getApiToken } from './api';

export interface SSEOptions {
  onEvent: (event: string, data: unknown) => void;
  onError?: (err: unknown) => void;
  onClose?: () => void;
  retry?: boolean;
  retryBaseMs?: number;
  retryMaxMs?: number;
}

export function connectSSE(path: string, options: SSEOptions): () => void {
  const controller = new AbortController();
  let stopped = false;
  let lastEventId = '';
  let retryAttempt = 0;
  const shouldRetry = options.retry ?? true;
  const retryBaseMs = options.retryBaseMs ?? 750;
  const retryMaxMs = options.retryMaxMs ?? 5000;

  function buildURL(): string {
    const url = new URL(`${getApiBaseUrl()}${path}`);
    if (lastEventId && !url.searchParams.has('after')) {
      url.searchParams.set('after', lastEventId);
    }
    return url.toString();
  }

  function delay(ms: number): Promise<void> {
    return new Promise(resolve => {
      const timeout = window.setTimeout(resolve, ms);
      controller.signal.addEventListener('abort', () => {
        window.clearTimeout(timeout);
        resolve();
      }, { once: true });
    });
  }

  function reconnectDelay(): number {
    const next = retryBaseMs * Math.pow(2, retryAttempt);
    retryAttempt += 1;
    return Math.min(retryMaxMs, next);
  }

  function dispatchEvent(event: string, dataLines: string[]) {
    if (dataLines.length === 0) return;
    const dataStr = dataLines.join('\n');
    try {
      const data = camelizeResponse(JSON.parse(dataStr));
      options.onEvent(event, data);
    } catch (err) {
      console.error('Error parsing SSE data:', err, dataStr);
    }
  }

  async function start() {
    while (!stopped) {
      try {
        const token = getApiToken();
        const headers = new Headers({ Accept: 'text/event-stream' });
        if (token) {
          headers.set('Authorization', `Bearer ${token}`);
        }
        if (lastEventId) {
          headers.set('Last-Event-ID', lastEventId);
        }

        const response = await fetch(buildURL(), {
          signal: controller.signal,
          headers,
        });

        if (!response.ok) {
          throw new Error(`SSE status ${response.status}`);
        }

        const reader = response.body?.getReader();
        if (!reader) {
          throw new Error('No response body reader');
        }

        retryAttempt = 0;
        const decoder = new TextDecoder();
        let buffer = '';
        let currentEvent = 'message';
        let dataLines: string[] = [];
        let pendingEventId = '';

        while (!stopped) {
          const { value, done } = await reader.read();
          if (done) {
            break;
          }

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';

          for (const rawLine of lines) {
            const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine;

            if (line === '') {
              dispatchEvent(currentEvent, dataLines);
              if (pendingEventId) {
                lastEventId = pendingEventId;
              }
              currentEvent = 'message';
              dataLines = [];
              pendingEventId = '';
              continue;
            }

            if (line.startsWith(':')) {
              continue;
            }

            const separatorIndex = line.indexOf(':');
            const field = separatorIndex >= 0 ? line.slice(0, separatorIndex) : line;
            const rawValue = separatorIndex >= 0 ? line.slice(separatorIndex + 1) : '';
            const valueText = rawValue.startsWith(' ') ? rawValue.slice(1) : rawValue;

            if (field === 'event') {
              currentEvent = valueText || 'message';
            } else if (field === 'data') {
              dataLines.push(valueText);
            } else if (field === 'id') {
              pendingEventId = valueText;
            }
          }
        }
      } catch (err: unknown) {
        const isAbort = err instanceof DOMException
          ? err.name === 'AbortError'
          : err instanceof Error && err.name === 'AbortError';
        if (!stopped && !isAbort && options.onError) {
          options.onError(err);
        }
      }

      if (!shouldRetry || stopped || controller.signal.aborted) {
        break;
      }

      await delay(reconnectDelay());
    }

    if (options.onClose) {
      options.onClose();
    }
  }

  if (typeof window === 'undefined') {
    return () => {
      stopped = true;
      controller.abort();
    };
  }

  start();

  return () => {
    stopped = true;
    controller.abort();
  };
}
