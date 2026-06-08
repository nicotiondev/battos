'use client';

import React, { useCallback, useState, useEffect, useRef } from 'react';
import { ApiError, apiClient } from '../lib/api';
import { Conversation, Message } from '../lib/types';
import { 
  Send, RefreshCw, MessageSquare, Plus, Bot, User, 
  ChevronRight, AlertCircle, DollarSign, X
} from 'lucide-react';

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

export default function NovaChat({ onClose }: { onClose?: () => void }) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string>('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [loading, setLoading] = useState(false);
  const [listLoading, setListLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState('');
  const [serviceUnavailable, setServiceUnavailable] = useState(false);
  const [showSessions, setShowSessions] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  const fetchConversations = useCallback(async () => {
    try {
      setListLoading(true);
      setServiceUnavailable(false);
      const normalized = await apiClient.listNovaCoreConversations() as Conversation[];
      setConversations(normalized.sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime()));
      if (normalized.length > 0 && !activeConvId) {
        setActiveConvId(normalized[0].id);
      }
    } catch (err) {
      console.error(err);
      setConversations([]);
      setServiceUnavailable(true);
      if (err instanceof ApiError && err.status === 503) {
        setErrorMsg('NovaCore no esta disponible porque la base de datos esta desconectada.');
      } else {
        setErrorMsg('No se pudo cargar NovaCore.');
      }
    } finally {
      setListLoading(false);
    }
  }, [activeConvId]);

  const fetchMessages = async (id: string) => {
    try {
      setLoading(true);
      const normalized = await apiClient.listNovaCoreMessages(id) as Message[];
      setMessages(normalized.sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()));
    } catch (err) {
      console.error(err);
      if (err instanceof ApiError && err.status === 503) {
        setServiceUnavailable(true);
        setErrorMsg('NovaCore no esta disponible porque la base de datos esta desconectada.');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchConversations();
  }, [fetchConversations]);

  useEffect(() => {
    if (activeConvId) {
      fetchMessages(activeConvId);
    } else {
      setMessages([]);
    }
  }, [activeConvId]);

  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages, loading]);

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || serviceUnavailable) return;
    const txt = inputText;
    setInputText('');
    setLoading(true);
    setErrorMsg('');

    // Añadir mensaje local del usuario de forma inmediata
    const userMsgDummy: Message = {
      id: Math.random().toString(),
      conversationId: activeConvId,
      role: 'user',
      content: txt,
      tokensIn: 0,
      tokensOut: 0,
      createdAt: new Date().toISOString()
    };
    setMessages(prev => [...prev, userMsgDummy]);

    try {
      setServiceUnavailable(false);
      const res = await apiClient.chatNovaCore({
        content: txt,
        conversationId: activeConvId || undefined
      });
      
      const assistMsg: Message = {
        id: Math.random().toString(),
        conversationId: res.conversationId,
        role: res.role,
        content: res.content,
        tokensIn: res.tokensIn,
        tokensOut: res.tokensOut,
        createdAt: new Date().toISOString()
      };

      setMessages(prev => {
        // Filtrar el dummy
        const filtered = prev.filter(m => m.id !== userMsgDummy.id);
        return [...filtered, { ...userMsgDummy, conversationId: res.conversationId }, assistMsg];
      });

      if (!activeConvId) {
        setActiveConvId(res.conversationId);
        fetchConversations();
      }
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al conectar con NovaCore"));
      if (err instanceof ApiError && err.status === 503) {
        setServiceUnavailable(true);
      }
      // Remover dummy si falló
      setMessages(prev => prev.filter(m => m.id !== userMsgDummy.id));
    } finally {
      setLoading(false);
    }
  };

  const handleStartNewChat = () => {
    setActiveConvId('');
    setMessages([]);
    setErrorMsg('');
  };

  // Calcular costo de la sesión activa
  const activeConv = conversations.find(c => c.id === activeConvId);

  return (
    <div className="flex flex-col h-full overflow-hidden border border-gray-800 rounded-xl bg-gray-950/60 relative">
      {/* Header del Chat — siempre visible */}
      <div className="p-3 border-b border-gray-800 flex justify-between items-center bg-gray-950 shrink-0 z-10">
        <div className="flex items-center gap-2">
          <button 
            onClick={() => setShowSessions(!showSessions)}
            className={`p-1.5 rounded border transition-all ${
              showSessions 
                ? 'bg-primary/20 border-primary/30 text-primary' 
                : 'bg-gray-900 border-gray-800 text-muted-foreground hover:text-white'
            }`}
            title="Historial de Sesiones"
          >
            <MessageSquare size={12} />
          </button>
          <Bot size={16} className="text-primary animate-pulse" />
          <div>
            <h4 className="text-xs font-bold text-white">NovaCore System AI</h4>
            <p className="text-[9px] text-muted-foreground">Orquestador de Sistema de BattOS</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {activeConv && (
            <div className="text-[9px] text-muted-foreground hidden sm:flex gap-3">
              <span>Tokens: <strong>{(activeConv.totalInputTokens + activeConv.totalOutputTokens)}</strong></span>
              <span className="text-emerald-400 flex items-center gap-0.5">
                <DollarSign size={10} /> {activeConv.totalCostUSD.toFixed(5)}
              </span>
            </div>
          )}
          {onClose && (
            <button 
              onClick={onClose}
              className="p-1 rounded bg-gray-900 border border-gray-800 text-muted-foreground hover:text-white hover:border-gray-700 transition-all"
              title="Cerrar Chat"
            >
              <X size={12} />
            </button>
          )}
        </div>
      </div>

      {/* Contenedor principal — zona de mensajes + overlay de sesiones */}
      <div className="flex-1 flex flex-col overflow-hidden relative">
        {serviceUnavailable && (
          <div className="m-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-[11px] text-amber-100">
            <div className="flex items-start gap-2">
              <AlertCircle size={14} className="mt-0.5 shrink-0 text-amber-300" />
              <div>
                <p className="font-bold text-amber-200">NovaCore en pausa</p>
                <p className="mt-1 text-amber-100/80">
                  El chat necesita la base SQLite local para conversaciones y puede requerir provider keys para responder. Reintenta cuando la DB este disponible.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Overlay de sesiones (cubre todo el área de mensajes) */}
        {showSessions && (
          <>
            {/* Backdrop para cerrar al hacer clic fuera */}
            <div 
              className="absolute inset-0 bg-black/40 z-10"
              onClick={() => setShowSessions(false)}
            />
            <div className="absolute inset-0 z-20 flex flex-col bg-gray-950/95 backdrop-blur-sm animate-in slide-in-from-left duration-200">
              <div className="p-3 border-b border-gray-800 flex items-center justify-between shrink-0">
                <span className="text-xs font-bold text-white flex items-center gap-1.5">
                  <MessageSquare size={14} className="text-primary" /> Sesiones Nova
                </span>
                <div className="flex items-center gap-1.5">
                  <button 
                    onClick={handleStartNewChat}
                    className="p-1 rounded bg-primary text-primary-foreground hover:bg-yellow-400"
                    title="Nuevo Chat"
                  >
                    <Plus size={12} />
                  </button>
                  <button 
                    onClick={() => setShowSessions(false)}
                    className="p-1 rounded bg-gray-900 border border-gray-800 text-muted-foreground hover:text-white transition-all"
                    title="Cerrar panel"
                  >
                    <X size={12} />
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto p-2 space-y-1">
                {conversations.map(c => (
                  <div 
                    key={c.id}
                    onClick={() => {
                      setActiveConvId(c.id);
                      setShowSessions(false);
                    }}
                    className={`p-3 rounded-lg cursor-pointer transition-all flex items-center justify-between text-left ${
                      activeConvId === c.id 
                        ? 'bg-primary/10 border border-primary/20 text-white' 
                        : 'hover:bg-gray-900/60 text-muted-foreground border border-transparent'
                    }`}
                  >
                    <div className="truncate pr-2">
                      <span className="text-[11px] font-mono block">
                        Chat {c.id.slice(0, 8)}
                      </span>
                      <span className="text-[9px] text-muted-foreground">
                        {new Date(c.startedAt).toLocaleDateString()} · {c.messageCount} msgs
                      </span>
                    </div>
                    <ChevronRight size={12} className="shrink-0" />
                  </div>
                ))}
                {conversations.length === 0 && !listLoading && (
                  <div className="text-[11px] text-muted-foreground italic text-center pt-12">
                    Sin conversaciones aún. Escribe un mensaje para empezar.
                  </div>
                )}
                {listLoading && (
                  <div className="flex items-center justify-center pt-12">
                    <RefreshCw className="animate-spin text-primary" size={16} />
                  </div>
                )}
              </div>
            </div>
          </>
        )}

        {errorMsg && (
          <div className="p-2.5 bg-red-500/10 border-b border-red-500/20 text-red-400 text-xs flex justify-between items-center shrink-0">
            <span className="flex items-center gap-1"><AlertCircle size={12} /> {errorMsg}</span>
            <button onClick={() => setErrorMsg('')} className="hover:text-white"><X size={12} /></button>
          </div>
        )}

        {/* Visor de Mensajes */}
        <div className="flex-1 p-4 overflow-y-auto space-y-4">
          {messages.map((m) => {
            const isUser = m.role === 'user';
            return (
              <div 
                key={m.id} 
                className={`flex gap-3 max-w-[85%] ${isUser ? 'ml-auto flex-row-reverse' : ''}`}
              >
                <div className={`p-2.5 rounded-lg ${
                  isUser ? 'bg-primary/10 text-white' : 'bg-gray-900/60 border border-gray-800 text-gray-200'
                }`}>
                  <div className="flex items-center gap-1.5 border-b border-gray-800/60 pb-1 mb-1.5 text-[9px] text-muted-foreground font-semibold">
                    {isUser ? (
                      <><User size={10} /> Usuario</>
                    ) : (
                      <><Bot size={10} className="text-primary" /> NovaCore</>
                    )}
                  </div>
                  <p className="text-xs whitespace-pre-wrap leading-relaxed">{m.content}</p>
                </div>
              </div>
            );
          })}
          {loading && (
            <div className="flex gap-2 items-center text-muted-foreground text-xs pl-2 animate-pulse">
              <RefreshCw className="animate-spin text-primary" size={12} />
              <span>NovaCore está pensando...</span>
            </div>
          )}
          {messages.length === 0 && !loading && (
            <div className="flex flex-col items-center justify-center h-full text-center p-6 space-y-3">
              <Bot size={36} className="text-primary" />
              <h4 className="text-xs font-bold text-white">¿En qué puedo ayudarte hoy?</h4>
              <p className="text-[10px] text-muted-foreground max-w-xs">
                Pregúntame acerca de proyectos, tareas, agentes o diagnosticar problemas en el OS.
              </p>
              <div className="grid grid-cols-1 gap-2 max-w-xs w-full pt-2">
                <button 
                  onClick={() => setInputText("¿Cuáles son mis proyectos activos?")}
                  className="p-2.5 bg-gray-900 hover:bg-gray-800 border border-gray-800 rounded text-[10px] text-left text-gray-300 transition-colors"
                >
                  💬 &quot;¿Cuáles son mis proyectos activos?&quot;
                </button>
                <button 
                  onClick={() => setInputText("¿Cómo está la salud del sistema?")}
                  className="p-2.5 bg-gray-900 hover:bg-gray-800 border border-gray-800 rounded text-[10px] text-left text-gray-300 transition-colors"
                >
                  🩺 &quot;¿Cómo está la salud del sistema?&quot;
                </button>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Bar */}
        <form onSubmit={handleSendMessage} className="p-3 border-t border-gray-800 bg-gray-950 flex gap-2 shrink-0">
          <input 
            type="text" 
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Chatea con NovaCore..."
            disabled={loading || serviceUnavailable}
            className="flex-1 bg-gray-900 border border-gray-800 rounded text-xs text-white px-3 py-2 focus:outline-none focus:border-primary disabled:opacity-50 min-w-0"
          />
          <button 
            type="submit" 
            disabled={loading || serviceUnavailable || !inputText.trim()}
            className="p-2 bg-primary text-primary-foreground hover:bg-yellow-400 rounded disabled:opacity-50 transition-all shrink-0"
          >
            <Send size={14} />
          </button>
        </form>
      </div>
    </div>
  );
}
