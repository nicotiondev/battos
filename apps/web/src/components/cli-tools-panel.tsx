'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '../lib/api';
import { CliTool } from '../lib/types';
import {
  Terminal, CheckCircle, XCircle, AlertCircle, Clock, Shield,
  Download, RotateCw, RefreshCw, ExternalLink, AlertTriangle, X, ChevronDown, ChevronUp,
} from 'lucide-react';

interface CliToolsPanelProps {
  /** Se invoca cuando una instalación exitosa terminó y se re-detectaron runtimes. */
  onDetectCompleted?: () => void;
}

type InstallPhase =
  | { phase: 'idle' }
  | { phase: 'submitting' }
  | { phase: 'installing'; installId: string }
  | { phase: 'succeeded' }
  | { phase: 'rejected'; reason?: string | null }
  | { phase: 'failed'; output: string | null };

const POLL_INTERVAL_MS = 2000;

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

function getStatusIcon(status: string) {
  switch (status) {
    case 'detected':
      return <CheckCircle size={12} className="text-emerald-400" />;
    case 'not_detected':
      return <XCircle size={12} className="text-gray-500" />;
    case 'broken':
      return <AlertCircle size={12} className="text-amber-400" />;
    default:
      return <AlertCircle size={12} className="text-gray-400" />;
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case 'detected':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold">
          Detectada
        </span>
      );
    case 'not_detected':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold">
          No detectada
        </span>
      );
    case 'broken':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-400 border border-amber-500/20 font-semibold">
          Rota
        </span>
      );
    default:
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold">
          {status}
        </span>
      );
  }
}

function getRiskBadge(riskLevel?: string) {
  if (!riskLevel) return null;
  const colors =
    riskLevel === 'high'
      ? 'bg-red-500/10 text-red-400 border-red-500/20'
      : riskLevel === 'medium'
        ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
        : 'bg-gray-500/10 text-gray-400 border-gray-500/20';
  return (
    <span className={`px-1 py-0.5 rounded text-[9px] border font-semibold uppercase ${colors}`}>
      riesgo {riskLevel}
    </span>
  );
}

export default function CliToolsPanel({ onDetectCompleted }: CliToolsPanelProps) {
  const [tools, setTools] = useState<CliTool[]>([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [installs, setInstalls] = useState<Record<string, InstallPhase>>({});
  const [confirmTool, setConfirmTool] = useState<CliTool | null>(null);
  const [expandedOutput, setExpandedOutput] = useState<Record<string, boolean>>({});

  const timersRef = useRef<Record<string, number>>({});
  const mountedRef = useRef(true);

  const setPhase = useCallback((toolId: string, phase: InstallPhase) => {
    setInstalls(prev => ({ ...prev, [toolId]: phase }));
  }, []);

  const stopPolling = useCallback((toolId: string) => {
    const timer = timersRef.current[toolId];
    if (timer !== undefined) {
      window.clearInterval(timer);
      delete timersRef.current[toolId];
    }
  }, []);

  const fetchTools = useCallback(async () => {
    try {
      setListError('');
      const items = await apiClient.listCliTools();
      if (!mountedRef.current) return;
      setTools(items);
    } catch (err) {
      console.error('Error fetching CLI tools', err);
      if (!mountedRef.current) return;
      setTools([]);
      setListError(errorMessage(err, 'No se pudo cargar el estado de las CLIs.'));
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    fetchTools();
    const timers = timersRef.current;
    return () => {
      mountedRef.current = false;
      Object.values(timers).forEach(timer => window.clearInterval(timer));
    };
  }, [fetchTools]);

  const handleInstallFinished = useCallback(async () => {
    // Re-detectar runtimes y refrescar la lista de CLIs tras una instalación exitosa.
    try {
      await apiClient.detectRuntimeAdapters();
    } catch (err) {
      console.error('Error re-detectando runtimes', err);
    }
    await fetchTools();
    if (onDetectCompleted) onDetectCompleted();
  }, [fetchTools, onDetectCompleted]);

  const startPolling = useCallback((toolId: string, installId: string) => {
    stopPolling(toolId);
    let busy = false;
    const tick = async () => {
      if (busy) return;
      busy = true;
      try {
        const list = await apiClient.listCliToolInstalls(toolId);
        const found = list.find(item => item.id === installId);
        if (!found || !mountedRef.current) return;
        if (found.status === 'succeeded') {
          stopPolling(toolId);
          setPhase(toolId, { phase: 'succeeded' });
          await handleInstallFinished();
        } else if (found.status === 'failed') {
          stopPolling(toolId);
          setPhase(toolId, { phase: 'failed', output: found.output ?? null });
        } else if (found.status === 'rejected') {
          stopPolling(toolId);
          setPhase(toolId, { phase: 'rejected', reason: found.reason });
        }
      } catch (err) {
        // Error transitorio de red: seguimos haciendo poll.
        console.error('Error consultando estado de instalación', err);
      } finally {
        busy = false;
      }
    };
    timersRef.current[toolId] = window.setInterval(tick, POLL_INTERVAL_MS);
  }, [handleInstallFinished, setPhase, stopPolling]);

  // Confirmar = recién acá se crea el install y se aprueba en el mismo flujo.
  const handleConfirmInstall = async () => {
    const tool = confirmTool;
    if (!tool) return;
    setConfirmTool(null);
    setPhase(tool.id, { phase: 'submitting' });
    try {
      const { install } = await apiClient.requestCliToolInstall(tool.id);
      const { install: decided } = await apiClient.approveCliToolInstall(install.id, {
        decision: 'approved',
        reason: 'Aprobado desde el dashboard',
      });
      if (decided.status === 'rejected') {
        setPhase(tool.id, { phase: 'rejected', reason: decided.reason });
        return;
      }
      setPhase(tool.id, { phase: 'installing', installId: install.id });
      startPolling(tool.id, install.id);
    } catch (err) {
      console.error('Error iniciando instalación', err);
      setPhase(tool.id, { phase: 'failed', output: errorMessage(err, 'Error al iniciar la instalación.') });
    }
  };

  const renderInstallActions = (tool: CliTool) => {
    const state = installs[tool.id] ?? { phase: 'idle' };
    const inFlight = state.phase === 'submitting' || state.phase === 'installing';

    if (inFlight) {
      return (
        <button
          disabled
          className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-semibold bg-primary/10 text-primary border border-primary/20 cursor-wait"
        >
          <RefreshCw size={10} className="animate-spin" /> Instalando…
        </button>
      );
    }

    const hasCommand = Boolean(tool.installCommand);
    const buttons: React.ReactNode[] = [];

    if (tool.status !== 'detected' && hasCommand) {
      buttons.push(
        <button
          key="install"
          onClick={() => setConfirmTool(tool)}
          className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-bold bg-primary text-primary-foreground hover:bg-yellow-400 transition-colors"
        >
          <Download size={10} /> Instalar
        </button>
      );
    }

    if (tool.status === 'detected' && hasCommand) {
      buttons.push(
        <button
          key="reinstall"
          onClick={() => setConfirmTool(tool)}
          title="Reinstalar"
          className="inline-flex items-center gap-1 px-1.5 py-1 rounded text-[10px] font-semibold text-muted-foreground border border-gray-800 hover:text-white hover:border-gray-700 transition-colors"
        >
          <RotateCw size={10} />
        </button>
      );
    }

    if (tool.installUrl) {
      buttons.push(
        <a
          key="docs"
          href={tool.installUrl}
          target="_blank"
          rel="noopener noreferrer"
          title="Documentación de instalación"
          className="inline-flex items-center gap-1 px-1.5 py-1 rounded text-[10px] font-semibold text-muted-foreground border border-gray-800 hover:text-white hover:border-gray-700 transition-colors"
        >
          <ExternalLink size={10} />
        </a>
      );
    }

    if (buttons.length === 0) return null;
    return <div className="flex items-center gap-1.5">{buttons}</div>;
  };

  const renderInstallResult = (tool: CliTool) => {
    const state = installs[tool.id];
    if (!state) return null;

    if (state.phase === 'succeeded') {
      return (
        <div className="mt-2 flex items-center gap-1.5 text-[10px] text-emerald-400">
          <CheckCircle size={10} /> Instalación completada. Runtimes re-detectados.
        </div>
      );
    }

    if (state.phase === 'rejected') {
      return (
        <div className="mt-2 flex items-center gap-1.5 text-[10px] text-amber-400">
          <AlertCircle size={10} /> Instalación rechazada{state.reason ? `: ${state.reason}` : '.'}
        </div>
      );
    }

    if (state.phase === 'failed') {
      const isOpen = Boolean(expandedOutput[tool.id]);
      return (
        <div className="mt-2 rounded-lg bg-red-500/10 border border-red-500/20 p-2 space-y-1.5">
          <button
            onClick={() => setExpandedOutput(prev => ({ ...prev, [tool.id]: !isOpen }))}
            className="w-full flex items-center justify-between text-[10px] font-semibold text-red-400"
          >
            <span className="flex items-center gap-1.5">
              <XCircle size={10} /> La instalación falló
            </span>
            {state.output ? (isOpen ? <ChevronUp size={10} /> : <ChevronDown size={10} />) : null}
          </button>
          {isOpen && state.output && (
            <pre className="max-h-32 overflow-y-auto whitespace-pre-wrap rounded bg-black/80 p-2 font-mono text-[9px] text-red-300">
              {state.output}
            </pre>
          )}
        </div>
      );
    }

    return null;
  };

  return (
    <div className="glass-panel rounded-xl border border-gray-800 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-bold text-white flex items-center gap-2">
          <Terminal size={14} className="text-primary" /> CLIs del Host
        </h3>
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-muted-foreground">{tools.length} herramientas</span>
          <button
            onClick={fetchTools}
            title="Refrescar estado de CLIs"
            className="p-1 rounded text-muted-foreground hover:text-white hover:bg-gray-900"
          >
            <RefreshCw size={11} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>
      </div>

      {listError && (
        <p className="text-[10px] text-red-400 mb-2">{listError}</p>
      )}

      {!loading && tools.length === 0 && !listError && (
        <p className="text-[10px] text-muted-foreground italic text-center py-3">
          No hay CLIs registradas en el sistema.
        </p>
      )}

      <div className="space-y-2">
        {tools.map(tool => (
          <div
            key={tool.id}
            className="p-2.5 rounded-lg bg-gray-900/50 border border-gray-800/60"
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-start gap-2 min-w-0">
                <div className="mt-0.5 shrink-0">{getStatusIcon(tool.status)}</div>
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <p className="text-[11px] font-semibold text-white truncate">{tool.name}</p>
                    {getStatusBadge(tool.status)}
                  </div>
                  <div className="flex flex-wrap items-center gap-1.5 mt-0.5">
                    <span className="font-mono text-[9px] text-muted-foreground bg-gray-950 px-1 py-0.5 rounded">
                      {tool.command}
                    </span>
                    {tool.version && (
                      <span className="text-[9px] text-muted-foreground">v{tool.version}</span>
                    )}
                    {getRiskBadge(tool.riskLevel)}
                    {tool.requiresAuth && (
                      <span className="flex items-center gap-0.5 text-[9px] text-amber-400">
                        <Shield size={8} /> auth
                      </span>
                    )}
                  </div>
                  {tool.status === 'detected' && tool.detectedPath && (
                    <p className="mt-0.5 font-mono text-[9px] text-muted-foreground truncate" title={tool.detectedPath}>
                      {tool.detectedPath}
                    </p>
                  )}
                  {tool.lastDetectedAt && (
                    <div className="flex items-center gap-0.5 mt-0.5">
                      <Clock size={8} className="text-muted-foreground" />
                      <span className="text-[9px] text-muted-foreground">
                        {new Date(tool.lastDetectedAt).toLocaleString('es', {
                          month: 'short', day: 'numeric',
                          hour: '2-digit', minute: '2-digit',
                        })}
                      </span>
                    </div>
                  )}
                </div>
              </div>
              <div className="shrink-0">{renderInstallActions(tool)}</div>
            </div>
            {renderInstallResult(tool)}
          </div>
        ))}
      </div>

      {/* Modal de confirmación de instalación (mutación del host) */}
      {confirmTool && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4 border border-gray-800">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Download size={16} className="text-primary" />
                {confirmTool.status === 'detected' ? 'Reinstalar' : 'Instalar'} {confirmTool.name}
              </h3>
              <button
                onClick={() => setConfirmTool(null)}
                className="text-muted-foreground hover:text-white"
              >
                <X size={16} />
              </button>
            </div>

            <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 flex gap-2.5 text-[11px] text-amber-100">
              <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-300" />
              <p>
                Este comando se va a ejecutar <strong className="text-amber-200">directamente en el host</strong> y
                modifica el sistema. Revisalo antes de confirmar.
              </p>
            </div>

            <div>
              <p className="text-[10px] text-muted-foreground uppercase font-semibold mb-1">
                Comando a ejecutar
              </p>
              <pre className="p-2.5 bg-black rounded-lg font-mono text-[11px] text-emerald-400 overflow-x-auto whitespace-pre-wrap break-all border border-gray-800">
                {confirmTool.installCommand}
              </pre>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button
                onClick={() => setConfirmTool(null)}
                className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded"
              >
                Cancelar
              </button>
              <button
                onClick={handleConfirmInstall}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded hover:bg-yellow-400"
              >
                <Download size={12} /> Confirmar e instalar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
