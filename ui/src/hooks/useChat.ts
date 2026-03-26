import { useState, useCallback, useEffect } from 'react';
import type { ChatMessage, ChatApiRequest, ChatApiResponse } from '../types';

const API_BASE = '/api';

export interface OpenAIModel {
  id: string;
  owned_by: string;
}

export function useChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previousResponseId, setPreviousResponseId] = useState<string | undefined>();
  const [models, setModels] = useState<OpenAIModel[]>([]);
  const [model, setModel] = useState('gpt-4o-mini');
  const [modelsLoading, setModelsLoading] = useState(true);

  const PREFERRED_DEFAULT = 'gpt-5-mini';
  const FALLBACK_DEFAULT = 'gpt-4o-mini';

  useEffect(() => {
    fetch(`${API_BASE}/chat/models`)
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then((data: OpenAIModel[]) => {
        setModels(data);
        if (data.some(m => m.id === PREFERRED_DEFAULT)) {
          setModel(PREFERRED_DEFAULT);
        } else if (data.some(m => m.id === FALLBACK_DEFAULT)) {
          setModel(FALLBACK_DEFAULT);
        } else if (data.length > 0) {
          setModel(data[0].id);
        }
      })
      .catch(() => setModels([]))
      .finally(() => setModelsLoading(false));
  }, []);

  const sendMessage = useCallback(async (content: string) => {
    const userMessage: ChatMessage = { role: 'user', content };
    setMessages(prev => [...prev, userMessage]);
    setLoading(true);
    setError(null);

    try {
      const body: ChatApiRequest = {
        message: content,
        model,
        previous_response_id: previousResponseId,
      };

      const response = await fetch(`${API_BASE}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const errData = await response.json().catch(() => null);
        throw new Error(errData?.detail || `API error: ${response.status}`);
      }

      const data: ChatApiResponse = await response.json();

      const assistantMessage: ChatMessage = {
        role: 'assistant',
        content: data.response,
        toolCalls: data.tool_calls.length > 0 ? data.tool_calls : undefined,
      };

      setMessages(prev => [...prev, assistantMessage]);
      setPreviousResponseId(data.response_id);
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : 'Unknown error';
      setError(errMsg);
      setMessages(prev => [
        ...prev,
        { role: 'assistant', content: `Error: ${errMsg}` },
      ]);
    } finally {
      setLoading(false);
    }
  }, [previousResponseId, model]);

  const clearChat = useCallback(() => {
    setMessages([]);
    setError(null);
    setPreviousResponseId(undefined);
  }, []);

  return { messages, loading, error, sendMessage, clearChat, models, model, setModel, modelsLoading };
}
