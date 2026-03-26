import { useState, useRef, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useChat } from '../hooks/useChat';
import type { ChatToolCall } from '../types';

const SUGGESTED_PROMPTS = [
  'How many active hosts are on my network?',
  'Which hosts have port 22 open?',
  'Show me recent network events',
  'What ports are open across the network?',
];

function ToolCallDetail({ toolCalls }: { toolCalls: ChatToolCall[] }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="mt-2">
      <button
        onClick={() => setExpanded(!expanded)}
        className="text-xs text-gray-500 hover:text-gray-700 flex items-center gap-1"
      >
        <svg
          className={`w-3 h-3 transition-transform ${expanded ? 'rotate-90' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
        {toolCalls.length} tool call{toolCalls.length !== 1 ? 's' : ''}
      </button>
      {expanded && (
        <div className="mt-1 space-y-1">
          {toolCalls.map((tc, i) => (
            <div key={i} className="text-xs bg-gray-50 rounded p-2 font-mono">
              <span className="text-blue-600">{tc.name}</span>
              <span className="text-gray-400">(</span>
              <span className="text-gray-600">
                {Object.keys(tc.arguments).length > 0
                  ? JSON.stringify(tc.arguments)
                  : ''}
              </span>
              <span className="text-gray-400">)</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function ChatPage() {
  const { messages, loading, sendMessage, clearChat, models, model, setModel, modelsLoading } = useChat();
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = input.trim();
    if (!trimmed || loading) return;
    setInput('');
    sendMessage(trimmed);
  };

  const handlePromptClick = (prompt: string) => {
    if (loading) return;
    sendMessage(prompt);
  };

  const isEmpty = messages.length === 0;

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 73px)' }}>
      {/* Header */}
      <div className="flex items-center justify-between py-3">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-gray-900">Network Assistant</h1>
          <select
            value={model}
            onChange={(e) => setModel(e.target.value)}
            disabled={modelsLoading || loading}
            className="text-sm border border-gray-300 rounded-md px-2 py-1 bg-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:bg-gray-50 disabled:text-gray-400"
          >
            {modelsLoading ? (
              <option>Loading models...</option>
            ) : models.length === 0 ? (
              <option>gpt-4o-mini</option>
            ) : (
              models.map((m) => (
                <option key={m.id} value={m.id}>{m.id}</option>
              ))
            )}
          </select>
        </div>
        {!isEmpty && (
          <button
            onClick={clearChat}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Clear chat
          </button>
        )}
      </div>

      {/* Messages area */}
      <div className="flex-1 overflow-y-auto pb-4">
        {isEmpty ? (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="text-gray-400 mb-6">
              <svg className="w-12 h-12 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z"
                />
              </svg>
              <p className="text-lg font-medium">Ask about your network</p>
              <p className="text-sm mt-1">I can look up hosts, ports, services, and events.</p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 max-w-lg">
              {SUGGESTED_PROMPTS.map((prompt) => (
                <button
                  key={prompt}
                  onClick={() => handlePromptClick(prompt)}
                  className="text-left text-sm px-4 py-3 bg-white border border-gray-200 rounded-lg hover:border-blue-300 hover:bg-blue-50 transition-colors"
                >
                  {prompt}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                <div
                  className={`max-w-[80%] rounded-lg px-4 py-3 ${
                    msg.role === 'user'
                      ? 'bg-blue-600 text-white'
                      : 'bg-white border border-gray-200 text-gray-900'
                  }`}
                >
                  {msg.role === 'assistant' ? (
                    <div className="text-sm prose prose-sm max-w-none prose-table:border-collapse prose-th:border prose-th:border-gray-300 prose-th:px-2 prose-th:py-1 prose-th:bg-gray-50 prose-td:border prose-td:border-gray-300 prose-td:px-2 prose-td:py-1 prose-p:my-1.5 prose-ul:my-1.5 prose-ol:my-1.5 prose-headings:my-2 prose-pre:bg-gray-800 prose-pre:text-gray-100 prose-code:text-pink-600 prose-code:before:content-none prose-code:after:content-none">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                    </div>
                  ) : (
                    <div className="text-sm whitespace-pre-wrap">{msg.content}</div>
                  )}
                  {msg.toolCalls && <ToolCallDetail toolCalls={msg.toolCalls} />}
                </div>
              </div>
            ))}
            {loading && (
              <div className="flex justify-start">
                <div className="bg-white border border-gray-200 rounded-lg px-4 py-3">
                  <div className="flex items-center gap-2 text-sm text-gray-500">
                    <div className="flex gap-1">
                      <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                      <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                      <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                    </div>
                    Thinking...
                  </div>
                </div>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-gray-200 bg-white py-3 -mx-4 px-4">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask about your network..."
            disabled={loading}
            className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:bg-gray-50 disabled:text-gray-400"
          />
          <button
            type="submit"
            disabled={loading || !input.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
          >
            Send
          </button>
        </form>
      </div>
    </div>
  );
}
