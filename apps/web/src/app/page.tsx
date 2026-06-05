'use client';

import React, { useState, useEffect, useRef } from 'react';
import { ApiError, clearApiToken, setApiToken, apiClient } from '../lib/api';
import { connectSSE } from '../lib/sse';
import { StatusResponse } from '../lib/types';
import DashboardView from '../components/DashboardView';
import WorkboardView from '../components/WorkboardView';
import AgentsView from '../components/AgentsView';
import ControlRoomView from '../components/ControlRoomView';
import KnowledgeView from '../components/KnowledgeView';
import NovaChat from '../components/NovaChat';
import UsageView from '../components/UsageView';
import SettingsView from '../components/SettingsView';
import { 
  Bot, LayoutDashboard, ClipboardList, Terminal as TermIcon, Database, 
  Settings, Key, Cpu, RefreshCw, Menu, DollarSign,
  ShieldAlert, LogOut, UserCog
} from 'lucide-react';

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function metricNumber(value: unknown, fallback: unknown): number {
  if (typeof value === 'number') return value;
  if (typeof fallback === 'number') return fallback;
  return 0;
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

function normalizeMetrics(data: unknown): StatusResponse | null {
  if (!data) return null;
  if (!isRecord(data)) return null;
  const rawMetrics = isRecord(data.metrics) ? data.metrics : {};
  return {
    ...data,
    metrics: {
      cpuPercent: metricNumber(rawMetrics.cpuPercent, rawMetrics.cpu_percent),
      memPercent: metricNumber(rawMetrics.memPercent, rawMetrics.mem_percent),
      memUsedMB: metricNumber(rawMetrics.memUsedMB, rawMetrics.memUsedMb ?? rawMetrics.mem_used_mb),
      memTotalMB: metricNumber(rawMetrics.memTotalMB, rawMetrics.memTotalMb ?? rawMetrics.mem_total_mb),
      netUploadKBps: metricNumber(rawMetrics.netUploadKBps, rawMetrics.netUploadKbps ?? rawMetrics.net_upload_kbps),
      netDownloadKBps: metricNumber(rawMetrics.netDownloadKBps, rawMetrics.netDownloadKbps ?? rawMetrics.net_download_kbps),
    }
  } as StatusResponse;
}

export default function Home() {
  const [activeTab, setActiveTab] = useState<'dashboard' | 'workboard' | 'agents' | 'controlroom' | 'knowledge' | 'usage' | 'settings'>('dashboard');
  const [tokenSet, setTokenSet] = useState(true);
  const [tokenInput, setTokenInput] = useState('');
  
  // Telemetría SSE
  const [metrics, setMetrics] = useState<StatusResponse | null>(null);
  const [apiOnline, setApiOnline] = useState(false);
  const [isNovaOpen, setIsNovaOpen] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

  // Terminal Interactiva Sidebar
  const [terminalInput, setTerminalInput] = useState('');
  const [terminalHistory, setTerminalHistory] = useState<string[]>([
    'nicotion@battos:~$ system --status',
    'OS Version:  1.0.0-alpha',
    'Kernel:      BattOS Core 1.0',
    'Overall:     ● OK (Api online)',
    'Type "help" to see available commands.',
    ''
  ]);
  const terminalEndRef = useRef<HTMLDivElement | null>(null);
  const databaseSubsystem = metrics?.subsystems?.find(sub => sub.name === 'database');
  const isSystemDegraded = !apiOnline || (metrics?.overall && metrics.overall !== 'ok');
  const systemStatusLabel = !apiOnline
    ? 'API offline'
    : metrics?.overall === 'down'
      ? 'Sistema degradado'
      : metrics?.overall === 'degraded'
        ? 'Sistema con advertencias'
        : 'Sistema operativo';
  const systemStatusDetail = !apiOnline
    ? 'No se pudo conectar con BattOS API. Revisa que localhost:8000 este corriendo.'
    : databaseSubsystem && databaseSubsystem.status !== 'ok'
      ? `Base de datos: ${databaseSubsystem.status}. ${databaseSubsystem.detail || 'Postgres no esta disponible.'}`
      : 'Todos los subsistemas principales estan respondiendo.';

  const fetchInitialMetrics = async () => {
    try {
      const data = await apiClient.getStatus();
      setMetrics(normalizeMetrics(data));
      setApiOnline(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setTokenSet(false);
      }
      setApiOnline(false);
    }
  };

  // Validar acceso al cargar. En dev local auth puede estar desactivada, por
  // eso primero intentamos entrar y solo pedimos token si la API responde 401.
  useEffect(() => {
    fetchInitialMetrics();
  }, []);

  // Conectar SSE de telemetría de sistema
  useEffect(() => {
    if (!tokenSet) return;

    const cleanup = connectSSE('/events/system-metrics', {
      onEvent: (event, data) => {
        if (event === 'system.metrics') {
          const m = data as StatusResponse;
          setMetrics(normalizeMetrics(m));
          setApiOnline(true);
        }
      },
      onError: (err) => {
        console.error("System telemetry SSE disconnected", err);
        setApiOnline(false);
      }
    });

    return () => cleanup();
  }, [tokenSet]);

  // Scroll terminal
  useEffect(() => {
    if (terminalEndRef.current) {
      terminalEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [terminalHistory]);

  const handleSaveToken = (e: React.FormEvent) => {
    e.preventDefault();
    if (!tokenInput.trim()) return;
    setApiToken(tokenInput);
    setTokenSet(true);
  };

  const handleLogout = () => {
    clearApiToken();
    setTokenSet(false);
    setMetrics(null);
    setApiOnline(false);
  };

  const handleTerminalSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cmd = terminalInput.trim();
    if (!cmd) return;

    setTerminalInput('');
    setTerminalHistory(prev => [...prev, `nicotion@battos:~$ ${cmd}`]);

    const args = cmd.split(' ');
    const mainCmd = args[0].toLowerCase();

    switch (mainCmd) {
      case 'clear':
        setTerminalHistory([]);
        break;
      case 'help':
        setTerminalHistory(prev => [
          ...prev,
          'Available commands:',
          '  status    - Show overall OS and metrics status',
          '  clear     - Clear terminal logs',
          '  ask <txt> - Send quick query to NovaCore assistant',
          ''
        ]);
        break;
      case 'status':
        if (metrics) {
          setTerminalHistory(prev => [
            ...prev,
            `Overall Health: ${metrics.overall.toUpperCase()}`,
            `CPU Usage:      ${metrics.metrics.cpuPercent.toFixed(1)}%`,
            `Memory RAM:     ${metrics.metrics.memPercent.toFixed(1)}%`,
            `Tráfico UP:     ${metrics.metrics.netUploadKBps.toFixed(1)} KB/s`,
            ''
          ]);
        } else {
          setTerminalHistory(prev => [...prev, 'Metrics offline.', '']);
        }
        break;
      case 'ask':
        const question = cmd.substring(4).trim();
        if (!question) {
          setTerminalHistory(prev => [...prev, 'Please ask a valid question. Usage: ask <question>', '']);
          break;
        }
        setTerminalHistory(prev => [...prev, 'Asking NovaCore...']);
        try {
          const res = await apiClient.chatNovaCore({ content: question });
          setTerminalHistory(prev => [...prev, `NovaCore > ${res.content}`, '']);
        } catch (err: unknown) {
          setTerminalHistory(prev => [...prev, `Error > ${errorMessage(err, 'connection failed')}`, '']);
        }
        break;
      default:
        setTerminalHistory(prev => [...prev, `Unknown command "${mainCmd}". Type "help" for commands.`, '']);
    }
  };

  if (!tokenSet) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#030712] px-4 font-sans">
        <div className="glass-panel w-full max-w-md p-8 rounded-2xl border border-gray-800 space-y-6 text-center">
          <div className="flex flex-col items-center gap-3">
            <div className="p-4 bg-primary/10 text-primary rounded-full neon-border-primary animate-pulse">
              <Bot size={40} />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-white mt-2">Bienvenido a BattOS</h1>
            <p className="text-xs text-muted-foreground max-w-xs">
              Ingresa el token administrador de acceso a la API configurado en tu servidor de BattOS para continuar.
            </p>
          </div>

          <form onSubmit={handleSaveToken} className="space-y-4">
            <div className="relative">
              <Key className="absolute left-3 top-3 text-muted-foreground" size={16} />
              <input 
                type="password"
                value={tokenInput}
                onChange={(e) => setTokenInput(e.target.value)}
                placeholder="Token de acceso (BATTOS_API_TOKEN)"
                required
                className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded-lg pl-10 p-3 focus:outline-none focus:border-primary"
              />
            </div>
            <button 
              type="submit" 
              className="w-full py-2.5 px-4 bg-primary text-primary-foreground font-semibold rounded-lg hover:bg-yellow-400 transition-all text-xs uppercase tracking-wider"
            >
              Conectarse al OS
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-[#030712] text-gray-200 overflow-hidden font-sans">
      {/* 1. Sidebar Izquierdo */}
      <div 
        className={`bg-gray-950/95 border-r border-gray-900 flex flex-col h-full transition-all duration-300 relative ${
          isSidebarOpen ? 'w-64' : 'w-0 overflow-hidden border-none'
        }`}
      >
        {/* Logo / Header Sidebar */}
        <div className="p-4 border-b border-gray-900 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="h-7 w-7 bg-primary text-primary-foreground font-bold rounded flex items-center justify-center text-sm shadow">
              B
            </div>
            <div>
              <h2 className="text-sm font-bold text-white tracking-wider flex items-center gap-1.5">
                BattOS <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              </h2>
              <p className="text-[9px] text-muted-foreground">AI Operating System</p>
            </div>
          </div>

          <button onClick={handleLogout} className="p-1 rounded hover:bg-gray-900 text-muted-foreground hover:text-white" title="Cerrar Sesión">
            <LogOut size={14} />
          </button>
        </div>

        {/* Links de Navegación */}
        <div className="flex-1 p-3 space-y-1 overflow-y-auto">
          <button 
            onClick={() => setActiveTab('dashboard')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'dashboard' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <LayoutDashboard size={14} /> Command Center
          </button>
          <button 
            onClick={() => setActiveTab('workboard')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'workboard' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <ClipboardList size={14} /> Work Board
          </button>
          <button 
            onClick={() => setActiveTab('agents')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'agents' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <UserCog size={14} /> Agents
          </button>
          <button 
            onClick={() => setActiveTab('controlroom')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'controlroom' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <TermIcon size={14} /> Control Room
          </button>
          <button 
            onClick={() => setActiveTab('knowledge')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'knowledge' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <Database size={14} /> Knowledge Center
          </button>
          <button 
            onClick={() => setActiveTab('usage')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'usage' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <DollarSign size={14} /> Usage & Limits
          </button>
          <button 
            onClick={() => setActiveTab('settings')}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'settings' ? 'bg-primary text-primary-foreground font-bold' : 'hover:bg-gray-900/60 text-muted-foreground hover:text-white'
            }`}
          >
            <Settings size={14} /> Settings
          </button>
        </div>

        {/* Mini Terminal Sidebar */}
        <div className="p-3 border-t border-gray-900 bg-black/40 flex flex-col h-48 overflow-hidden">
          <div className="flex items-center justify-between text-[9px] text-muted-foreground uppercase font-semibold pb-1.5 border-b border-gray-900">
            <span>Terminal de Sistema</span>
            {apiOnline ? (
              <span className="text-emerald-500 flex items-center gap-0.5">● Online</span>
            ) : (
              <span className="text-rose-500 flex items-center gap-0.5">● Offline</span>
            )}
          </div>
          
          <div className="flex-1 overflow-y-auto font-mono text-[9px] text-emerald-400 p-1.5 space-y-1 min-h-0">
            {terminalHistory.map((line, idx) => (
              <div key={idx} className="whitespace-pre-wrap leading-tight">{line}</div>
            ))}
            <div ref={terminalEndRef} />
          </div>

          <form onSubmit={handleTerminalSubmit} className="mt-1 border-t border-gray-900 pt-1 flex gap-1">
            <span className="font-mono text-[9px] text-primary select-none">$</span>
            <input 
              type="text"
              value={terminalInput}
              onChange={(e) => setTerminalInput(e.target.value)}
              placeholder="Type command..."
              className="flex-1 bg-transparent font-mono text-[9px] text-emerald-400 focus:outline-none"
            />
          </form>
        </div>
      </div>

      {/* 2. Área Central del Tablero */}
      <div className="flex-1 flex flex-col h-full overflow-hidden bg-radial-gradient">
        {/* Header Superior */}
        <header className="h-14 border-b border-gray-900 bg-gray-950/80 backdrop-blur-md px-6 flex items-center justify-between z-10">
          <div className="flex items-center gap-3">
            <button 
              onClick={() => setIsSidebarOpen(!isSidebarOpen)}
              className="p-1.5 rounded bg-gray-900 border border-gray-800 text-muted-foreground hover:text-white"
            >
              <Menu size={14} />
            </button>
            <h1 className="text-sm font-bold text-white uppercase tracking-wider">
              {activeTab === 'dashboard' && 'Command Center'}
              {activeTab === 'workboard' && 'Work Board'}
              {activeTab === 'agents' && 'Agents Registry'}
              {activeTab === 'controlroom' && 'Control Room'}
              {activeTab === 'knowledge' && 'Knowledge Center'}
              {activeTab === 'usage' && 'Usage & Limits'}
              {activeTab === 'settings' && 'Settings'}
            </h1>
          </div>

          {/* Widgets de Telemetría Rápida Header */}
          <div className="flex items-center gap-4 text-xs">
            {metrics && (
              <div className="hidden md:flex items-center gap-4 bg-gray-900/60 border border-gray-800/80 px-4 py-1.5 rounded-full text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Cpu size={12} className="text-emerald-400" />
                  <span>CPU: <strong className="text-white">{metrics.metrics.cpuPercent.toFixed(0)}%</strong></span>
                </div>
                <span>|</span>
                <div className="flex items-center gap-1.5">
                  <Database size={12} className="text-indigo-400" />
                  <span>RAM: <strong className="text-white">{metrics.metrics.memPercent.toFixed(0)}%</strong></span>
                </div>
              </div>
            )}

            <button 
              onClick={() => setIsNovaOpen(!isNovaOpen)}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-semibold transition-all ${
                isNovaOpen 
                  ? 'bg-primary/20 text-primary border border-primary/30' 
                  : 'bg-gray-900 hover:bg-gray-800 text-muted-foreground border border-gray-800'
              }`}
            >
              <Bot size={12} /> NovaCore
            </button>
          </div>
        </header>

        {/* Contenido Central de las Pestañas */}
        <main className="flex-1 overflow-y-auto p-6">
          {isSystemDegraded && (
            <div className="mb-5 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-100 shadow-lg shadow-amber-950/20">
              <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                <div className="flex gap-3">
                  <ShieldAlert size={18} className="mt-0.5 shrink-0 text-amber-300" />
                  <div>
                    <p className="font-bold uppercase tracking-wide text-amber-200">{systemStatusLabel}</p>
                    <p className="mt-1 max-w-4xl text-amber-100/80">{systemStatusDetail}</p>
                  </div>
                </div>
                <button
                  onClick={fetchInitialMetrics}
                  className="inline-flex items-center justify-center gap-1.5 rounded border border-amber-400/30 bg-black/20 px-3 py-1.5 font-semibold text-amber-100 hover:bg-black/30"
                >
                  <RefreshCw size={12} /> Reintentar
                </button>
              </div>
            </div>
          )}

          {activeTab === 'dashboard' && (
            <DashboardView metrics={metrics} />
          )}
          {activeTab === 'workboard' && <WorkboardView />}
          {activeTab === 'agents' && <AgentsView />}
          {activeTab === 'controlroom' && <ControlRoomView />}
          {activeTab === 'knowledge' && <KnowledgeView />}
          {activeTab === 'usage' && <UsageView />}
          {activeTab === 'settings' && (
            <SettingsView metrics={metrics} apiOnline={apiOnline} onRefresh={fetchInitialMetrics} />
          )}
        </main>
      </div>

      {/* Capa de fondo oscura clicable para colapsar cuando el chat está abierto */}
      {isNovaOpen && (
        <div 
          onClick={() => setIsNovaOpen(false)}
          className="fixed inset-0 bg-black/60 backdrop-blur-xs z-30 transition-opacity"
        />
      )}

      {/* 3. Sidebar Derecho: Chat NovaCore (Drawer flotante responsivo) */}
      <div 
        className={`bg-gray-950 border-l border-gray-900 flex flex-col h-full fixed right-0 top-0 bottom-0 z-40 shadow-2xl transition-all duration-300 transform ${
          isNovaOpen ? 'w-full sm:w-96 translate-x-0' : 'w-0 overflow-hidden border-none translate-x-full'
        }`}
      >
        <NovaChat onClose={() => setIsNovaOpen(false)} />
      </div>
    </div>
  );
}
