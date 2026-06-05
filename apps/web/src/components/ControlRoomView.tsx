'use client';

import React, { useState, useEffect, useRef } from 'react';
import { ApiError, apiClient } from '../lib/api';
import { connectSSE } from '../lib/sse';
import { Run, Project, Task, Agent, AgentRuntime, RunLog, Repository } from '../lib/types';
import { 
  Play, StopCircle, Terminal, Check, X, Plus,
  Clock, GitBranch, RefreshCw, Cpu, Globe, Database, CheckSquare, AlertTriangle
} from 'lucide-react';

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

type RunnableAdapterID = 'claude-code' | 'codex' | 'sandbox-smoke' | 'sandbox-memory-smoke';
type ApprovalKind = 'execute' | 'network' | 'commit' | 'push' | 'remember';

function isRunnableAdapterID(value: string): value is RunnableAdapterID {
  return value === 'claude-code' || value === 'codex' || value === 'sandbox-smoke' || value === 'sandbox-memory-smoke';
}

export default function ControlRoomView() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [selectedRun, setSelectedRun] = useState<Run | null>(null);
  const [logs, setLogs] = useState<RunLog[]>([]);
  const [diff, setDiff] = useState<string>('');
  
  // Proponer nuevo run
  const [showNewRunModal, setShowNewRunModal] = useState(false);
  const [projects, setProjects] = useState<Project[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [runtimes, setRuntimes] = useState<AgentRuntime[]>([]);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [newRunForm, setNewRunForm] = useState({
    projectId: '', taskId: '', agentId: '', skillId: '', runtimeAdapterId: '', repositoryId: '', prompt: '', requestedNetwork: false
  });

  const [errorMsg, setErrorMsg] = useState('');
  const [dataUnavailable, setDataUnavailable] = useState(false);
  
  const sseCleanupRef = useRef<(() => void) | null>(null);
  const logsEndRef = useRef<HTMLDivElement | null>(null);
  const selectedRunId = selectedRun?.id;
  const selectedRunStatus = selectedRun?.status;

  const fetchRuns = async () => {
    try {
      setErrorMsg('');
      setDataUnavailable(false);
      const items = await apiClient.listRuns();
      // Ordenar cronológicamente descendiente
      setRuns((items as Run[]).sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()));
    } catch (err) {
      console.error("Error fetching runs", err);
      setRuns([]);
      setSelectedRun(null);
      setDataUnavailable(true);
      if (err instanceof ApiError && err.status === 503) {
        setErrorMsg("Control Room no esta disponible porque la base de datos esta desconectada.");
      } else {
        setErrorMsg("No se pudo cargar el historial de runs.");
      }
    }
  };

  const fetchStaticData = async () => {
    try {
      const [projs, tks, ags, rts, repos] = await Promise.all([
        apiClient.listProjects(),
        apiClient.listTasks(),
        apiClient.listAgents(),
        apiClient.listRuntimeAdapters(),
        apiClient.listRepositories()
      ]);
      setProjects(projs as Project[]);
      setTasks(tks as Task[]);
      setAgents(ags as Agent[]);
      setRuntimes(rts as AgentRuntime[]);
      setRepositories(repos as Repository[]);
      if (projs.length > 0) {
        setNewRunForm(prev => ({ 
          ...prev, 
          projectId: projs[0].id,
          runtimeAdapterId: rts[0]?.id || ''
        }));
      }
    } catch (err) {
      console.error(err);
      setProjects([]);
      setTasks([]);
      setAgents([]);
      setRuntimes([]);
      setRepositories([]);
      setDataUnavailable(true);
    }
  };

  useEffect(() => {
    fetchRuns();
    fetchStaticData();
    return () => {
      if (sseCleanupRef.current) sseCleanupRef.current();
    };
  }, []);

  // Escuchar logs en caliente del run seleccionado
  useEffect(() => {
    if (sseCleanupRef.current) {
      sseCleanupRef.current();
      sseCleanupRef.current = null;
    }

    if (!selectedRunId) {
      setLogs([]);
      setDiff('');
      return;
    }

    // Cargar logs iniciales
    apiClient.listRunLogs(selectedRunId)
      .then(items => setLogs(items.sort((a, b) => a.id - b.id)))
      .catch(err => console.error(err));

    // Cargar diff si el run es terminal
    if (selectedRunStatus === 'succeeded') {
      apiClient.getRunDiff(selectedRunId)
        .then(res => setDiff(res))
        .catch(() => setDiff(''));
    }

    // Si no es terminal, conectar stream SSE
    const isTerminal = ['succeeded', 'failed', 'cancelled'].includes(selectedRunStatus || '');
    if (!isTerminal) {
      const cleanup = connectSSE(`/events/runs/${selectedRunId}`, {
        onEvent: (event, data) => {
          if (event === 'run.log') {
            const newLog = data as RunLog;
            setLogs(prev => {
              // Deduplicar por si acaso
              if (prev.some(l => l.id === newLog.id)) return prev;
              return [...prev, newLog].sort((a, b) => a.id - b.id);
            });
          } else if (event === 'run.snapshot') {
            const snap = data as Run;
            setSelectedRun(snap);
            // Actualizar en el listado
            setRuns(prev => prev.map(r => r.id === snap.id ? snap : r));
          } else if (event === 'run.done') {
            const snap = data as Run;
            setSelectedRun(snap);
            setRuns(prev => prev.map(r => r.id === snap.id ? snap : r));
            // Cargar diff al terminar
            apiClient.getRunDiff(snap.id)
              .then(res => setDiff(res))
              .catch(() => {});
            if (sseCleanupRef.current) {
              sseCleanupRef.current();
              sseCleanupRef.current = null;
            }
          }
        },
        onError: (err) => {
          console.error("SSE connection error", err);
        }
      });
      sseCleanupRef.current = cleanup;
    }
  }, [selectedRunId, selectedRunStatus]);

  // Auto scroll logs
  useEffect(() => {
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs]);

  const handleProposeRun = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newRunForm.projectId || !newRunForm.taskId || !newRunForm.agentId || !newRunForm.prompt) return;
    if (!isRunnableAdapterID(newRunForm.runtimeAdapterId)) {
      setErrorMsg('Selecciona un runtime ejecutable aprobado para v0.1.');
      return;
    }
    try {
      const res = await apiClient.proposeRun({
        projectId: newRunForm.projectId,
        taskId: newRunForm.taskId,
        agentId: newRunForm.agentId,
        skillId: newRunForm.skillId || undefined,
        runtimeAdapterId: newRunForm.runtimeAdapterId,
        repositoryId: newRunForm.repositoryId || undefined,
        prompt: newRunForm.prompt,
        requestedNetwork: newRunForm.requestedNetwork
      }) as Run;
      setShowNewRunModal(false);
      setNewRunForm({
        projectId: projects[0]?.id || '',
        taskId: '',
        agentId: '',
        skillId: '',
        runtimeAdapterId: runtimes[0]?.id || '',
        repositoryId: '',
        prompt: '',
        requestedNetwork: false
      });
      fetchRuns();
      setSelectedRun(res);
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al proponer ejecución"));
    }
  };

  const handleApproval = async (kind: ApprovalKind, decision: 'approved' | 'rejected', reason: string = 'Aprobado desde Dashboard') => {
    if (!selectedRun) return;
    try {
      await apiClient.approveRunAction(selectedRun.id, {
        kind,
        decision,
        reason
      });
      // Refrescar estado del run
      const updated = await apiClient.getRun(selectedRun.id) as Run;
      setSelectedRun(updated);
      setRuns(prev => prev.map(r => r.id === updated.id ? updated : r));
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al procesar aprobación"));
    }
  };

  const handleCancelRun = async () => {
    if (!selectedRun) return;
    try {
      await apiClient.cancelRun(selectedRun.id);
      const updated = await apiClient.getRun(selectedRun.id) as Run;
      setSelectedRun(updated);
      setRuns(prev => prev.map(r => r.id === updated.id ? updated : r));
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al cancelar ejecución"));
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'succeeded':
        return <span className="px-2 py-0.5 rounded text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold">Exitoso</span>;
      case 'failed':
        return <span className="px-2 py-0.5 rounded text-[10px] bg-red-500/10 text-red-400 border border-red-500/20 font-semibold">Fallido</span>;
      case 'running':
        return <span className="px-2 py-0.5 rounded text-[10px] bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 font-semibold animate-pulse">Ejecutando</span>;
      case 'queued':
        return <span className="px-2 py-0.5 rounded text-[10px] bg-blue-500/10 text-blue-400 border border-blue-500/20 font-semibold">En Cola</span>;
      case 'awaiting_approval':
        return <span className="px-2 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-400 border border-amber-500/20 font-semibold">Esperando HITL</span>;
      default:
        return <span className="px-2 py-0.5 rounded text-[10px] bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold">{status}</span>;
    }
  };

  const uniqueProjects = Array.from(new Map(projects.map(p => [p.id, p])).values());

  return (
    <div className="grid grid-cols-1 lg:grid-cols-4 gap-6 h-[76vh]">
      {/* 1. Panel Izquierdo: Listado de Runs */}
      <div className="glass-panel rounded-xl flex flex-col h-full overflow-hidden border border-gray-800">
        <div className="p-4 border-b border-gray-800 flex items-center justify-between">
          <h3 className="text-sm font-bold text-white flex items-center gap-2">
            <Clock size={16} className="text-primary" /> Historial de Runs
          </h3>
          <button 
            onClick={() => setShowNewRunModal(true)}
            className="p-1.5 rounded bg-primary text-primary-foreground hover:bg-yellow-400"
            title="Proponer Run"
            disabled={dataUnavailable}
          >
            <Plus size={14} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto divide-y divide-gray-800/60 p-2 space-y-1">
          {runs.map(r => (
            <div 
              key={r.id} 
              onClick={() => setSelectedRun(r)}
              className={`p-3 rounded-lg cursor-pointer transition-all ${
                selectedRun?.id === r.id 
                  ? 'bg-primary/10 border border-primary/20' 
                  : 'hover:bg-gray-900/30 border border-transparent'
              }`}
            >
              <div className="flex justify-between items-start gap-1">
                <span className="text-[10px] font-mono text-muted-foreground truncate w-24">
                  {r.id.slice(0, 8)}...
                </span>
                {getStatusBadge(r.status)}
              </div>
              <h4 className="text-xs font-bold text-white mt-1.5 truncate">{r.prompt}</h4>
              <div className="flex items-center gap-1.5 mt-2 text-[9px] text-muted-foreground">
                <span className="px-1 py-0.2 rounded bg-gray-900">{r.projectId}</span>
                <span>•</span>
                <span>{r.agentId}</span>
              </div>
            </div>
          ))}
          {runs.length === 0 && (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground italic">
              No hay runs en el sistema.
            </div>
          )}
        </div>
      </div>

      {/* 2. Panel Derecho: Detalle, Logs SSE, Diff y HITL Panel */}
      <div className="lg:col-span-3 flex flex-col h-full overflow-hidden space-y-4">
        {dataUnavailable && (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-100">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="flex gap-3">
                <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
                <div>
                  <p className="font-bold text-amber-200">Control Room no disponible</p>
                  <p className="mt-1 text-amber-100/80">
                    Runs, approvals, logs y diffs dependen de Postgres. Cuando la DB vuelva, esta pantalla podra proponer y supervisar ejecuciones.
                  </p>
                </div>
              </div>
              <button
                onClick={() => { fetchRuns(); fetchStaticData(); }}
                className="inline-flex items-center justify-center gap-1.5 rounded border border-amber-400/30 bg-black/20 px-3 py-1.5 font-semibold text-amber-100 hover:bg-black/30"
              >
                <RefreshCw size={12} /> Reintentar
              </button>
            </div>
          </div>
        )}

        {errorMsg && (
          <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-lg flex justify-between items-center">
            <span>{errorMsg}</span>
            <button onClick={() => setErrorMsg('')} className="hover:text-white"><X size={14} /></button>
          </div>
        )}

        {selectedRun ? (
          <div className="flex-1 flex flex-col overflow-hidden space-y-4">
            {/* Header Detalle Run */}
            <div className="glass-panel p-4 rounded-xl border border-gray-800 flex flex-col md:flex-row md:items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-mono bg-gray-950 px-2 py-0.5 rounded text-gray-300">Run: {selectedRun.id}</span>
                  {getStatusBadge(selectedRun.status)}
                </div>
                <h3 className="text-sm font-bold text-white mt-2">&quot;{selectedRun.prompt}&quot;</h3>
                <div className="flex flex-wrap items-center gap-3 mt-1.5 text-[10px] text-muted-foreground">
                  <span>Proyecto: <strong className="text-gray-300">{selectedRun.projectId}</strong></span>
                  <span>•</span>
                  <span>Tarea: <strong className="text-gray-300">{selectedRun.taskId}</strong></span>
                  <span>•</span>
                  <span>Agente: <strong className="text-indigo-400">{selectedRun.agentId}</strong></span>
                  <span>•</span>
                  <span>Runtime: <strong className="text-emerald-400">{selectedRun.runtimeAdapterId}</strong></span>
                </div>
              </div>

              <div className="flex gap-2">
                {/* Botón de Cancelación */}
                {!['succeeded', 'failed', 'cancelled'].includes(selectedRun.status) && (
                  <button 
                    onClick={handleCancelRun}
                    className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-semibold rounded bg-red-500/10 text-red-400 border border-red-500/20 hover:bg-red-500/20"
                  >
                    <StopCircle size={14} /> Cancelar Run
                  </button>
                )}
              </div>
            </div>

            {/* Panel Central del Detalle: Logs SSE y Diff / Metadata */}
            <div className="flex-1 grid grid-cols-1 lg:grid-cols-3 gap-4 overflow-hidden">
              {/* Visor de Consola (SSE Logs) */}
              <div className="lg:col-span-2 glass-panel rounded-xl flex flex-col overflow-hidden border border-gray-800">
                <div className="p-3 border-b border-gray-800 bg-gray-900/40 flex items-center justify-between">
                  <span className="text-xs font-bold text-white flex items-center gap-2">
                    <Terminal size={14} className="text-primary" /> Consola de Logs (SSE)
                  </span>
                  {!['succeeded', 'failed', 'cancelled'].includes(selectedRun.status) && (
                    <RefreshCw className="animate-spin text-primary" size={12} />
                  )}
                </div>
                <div className="flex-1 p-4 bg-black/95 font-mono text-[11px] text-emerald-400 overflow-y-auto space-y-1.5">
                  {logs.map((log) => {
                    let color = 'text-emerald-400';
                    if (log.stream === 'stderr') color = 'text-rose-400';
                    if (log.stream === 'system') color = 'text-cyan-400';
                    return (
                      <div key={log.id} className="whitespace-pre-wrap">
                        <span className="text-[10px] text-muted-foreground mr-2 font-sans">
                          [{log.stream.toUpperCase()}]
                        </span>
                        <span className={color}>{log.message}</span>
                      </div>
                    );
                  })}
                  {logs.length === 0 && (
                    <div className="text-muted-foreground italic text-center pt-8">
                      Iniciando y montando sandbox... esperando logs.
                    </div>
                  )}
                  <div ref={logsEndRef} />
                </div>
              </div>

              {/* Lado Derecho del Detalle: Diff & HITL Panel */}
              <div className="flex flex-col space-y-4 overflow-y-auto pr-1">
                {/* Panel de Aprobación Humana (HITL) */}
                <div className="glass-panel p-4 rounded-xl border border-gray-800 space-y-4">
                  <h4 className="text-xs font-bold text-white flex items-center gap-1.5">
                    <CheckSquare size={14} className="text-primary" /> Panel de Supervisión (HITL)
                  </h4>

                  {selectedRun.status === 'awaiting_approval' && (
                    <div className="space-y-3 p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
                      <p className="text-[10px] text-amber-300 font-medium">El run requiere aprobación para ejecutarse.</p>
                      <div className="flex gap-2">
                        <button 
                          onClick={() => handleApproval('execute', 'approved')}
                          className="flex-1 py-1 px-2 rounded text-xs font-bold bg-emerald-500 text-white hover:bg-emerald-600 flex items-center justify-center gap-1"
                        >
                          <Check size={12} /> Aprobar
                        </button>
                        <button 
                          onClick={() => handleApproval('execute', 'rejected')}
                          className="py-1 px-2 rounded text-xs font-bold bg-red-500/10 text-red-400 border border-red-500/20 hover:bg-red-500/20"
                        >
                          Rechazar
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Aprobaciones de Git en Caliente */}
                  {selectedRun.status === 'succeeded' && (
                    <div className="space-y-3">
                      <div className="p-3 rounded-lg bg-gray-900/60 border border-gray-800 space-y-2">
                        <p className="text-[10px] text-gray-300">Cambios Git listos. ¿Deseas hacer commit?</p>
                        <button 
                          onClick={() => handleApproval('commit', 'approved')}
                          className="w-full py-1.5 px-2 rounded text-xs font-semibold bg-indigo-600 text-white hover:bg-indigo-700 flex items-center justify-center gap-1"
                        >
                          <GitBranch size={12} /> Confirmar Commit
                        </button>
                      </div>
                      <div className="p-3 rounded-lg bg-gray-900/60 border border-gray-800 space-y-2">
                        <p className="text-[10px] text-gray-300">¿Deseas subir los cambios al origen (push)?</p>
                        <button 
                          onClick={() => handleApproval('push', 'approved')}
                          className="w-full py-1.5 px-2 rounded text-xs font-semibold bg-emerald-600 text-white hover:bg-emerald-700 flex items-center justify-center gap-1"
                        >
                          <Globe size={12} /> Subir Cambios (Push)
                        </button>
                      </div>
                      <div className="p-3 rounded-lg bg-gray-900/60 border border-gray-800 space-y-2">
                        <p className="text-[10px] text-gray-300">¿Guardar aprendizaje en memoria?</p>
                        <button 
                          onClick={() => handleApproval('remember', 'approved')}
                          className="w-full py-1.5 px-2 rounded text-xs font-semibold bg-amber-500 text-black hover:bg-amber-400 flex items-center justify-center gap-1"
                        >
                          <Database size={12} /> Recordar (Remember)
                        </button>
                      </div>
                    </div>
                  )}

                  {selectedRun.status !== 'awaiting_approval' && selectedRun.status !== 'succeeded' && (
                    <p className="text-[10px] text-muted-foreground italic text-center py-2">
                      No hay acciones pendientes de aprobación en el estado actual.
                    </p>
                  )}
                </div>

                {/* Git Diff Viewer */}
                {diff && (
                  <div className="glass-panel p-4 rounded-xl border border-gray-800 space-y-2">
                    <h4 className="text-xs font-bold text-white flex items-center gap-1.5">
                      <GitBranch size={14} className="text-primary" /> Git Diff Calculado
                    </h4>
                    <div className="p-2.5 bg-black rounded font-mono text-[9px] text-gray-300 overflow-x-auto whitespace-pre h-40 max-h-40">
                      {diff}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 glass-panel rounded-xl border border-gray-800 flex flex-col items-center justify-center text-center p-8">
            <Cpu size={48} className="text-muted-foreground mb-4" />
            <h4 className="text-sm font-bold text-white">Ningún Run Seleccionado</h4>
            <p className="text-xs text-muted-foreground max-w-sm mt-1">
              Selecciona una ejecución de run de la lista izquierda para visualizar su estado, logs SSE en vivo y aprobaciones HITL.
            </p>
          </div>
        )}
      </div>

      {/* 3. Modal: Proponer Nuevo Run */}
      {showNewRunModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Play size={16} className="text-primary" /> Proponer Nueva Ejecución (Run)
              </h3>
              <button onClick={() => setShowNewRunModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleProposeRun} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Proyecto</label>
                <select 
                  value={newRunForm.projectId} 
                  onChange={(e) => setNewRunForm({ ...newRunForm, projectId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                  required
                >
                  <option value="">Selecciona Proyecto</option>
                  {uniqueProjects.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Tarea Asociada</label>
                <select 
                  value={newRunForm.taskId} 
                  onChange={(e) => setNewRunForm({ ...newRunForm, taskId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                  required
                >
                  <option value="">Selecciona Tarea</option>
                  {tasks.filter(t => t.projectId === newRunForm.projectId).map(t => (
                    <option key={t.id} value={t.id}>{t.title} ({t.status})</option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Agente Ejecutor</label>
                  <select 
                    value={newRunForm.agentId} 
                    onChange={(e) => setNewRunForm({ ...newRunForm, agentId: e.target.value })}
                    className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                    required
                  >
                    <option value="">Selecciona Agente</option>
                    {agents.map(a => (
                      <option key={a.id} value={a.id}>{a.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Runtime Adapter</label>
                  <select 
                    value={newRunForm.runtimeAdapterId} 
                    onChange={(e) => setNewRunForm({ ...newRunForm, runtimeAdapterId: e.target.value })}
                    className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                    required
                  >
                    <option value="">Selecciona Runtime</option>
                    {runtimes.map(r => (
                      <option key={r.id} value={r.id}>{r.name} ({r.command || r.status})</option>
                    ))}
                  </select>
                </div>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Repositorio Opcional</label>
                <select
                  value={newRunForm.repositoryId}
                  onChange={(e) => setNewRunForm({ ...newRunForm, repositoryId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="">Sin repositorio conectado</option>
                  {repositories.filter(repo => repo.projectId === newRunForm.projectId).map(repo => (
                    <option key={repo.id} value={repo.id}>
                      {repo.name} ({repo.kind} / {repo.defaultBranch})
                    </option>
                  ))}
                </select>
                <p className="mt-1 text-[10px] text-muted-foreground">
                  Si eliges un repo, BattOS clona/aisla el workspace y guarda diff del run.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <input 
                  type="checkbox" 
                  id="requestedNetwork"
                  checked={newRunForm.requestedNetwork} 
                  onChange={(e) => setNewRunForm({ ...newRunForm, requestedNetwork: e.target.checked })}
                  className="bg-gray-950 border border-gray-800 rounded focus:ring-primary"
                />
                <label htmlFor="requestedNetwork" className="text-[10px] text-muted-foreground uppercase font-semibold">Solicitar Acceso a Red</label>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Instrucción (Prompt)</label>
                <textarea 
                  value={newRunForm.prompt} 
                  onChange={(e) => setNewRunForm({ ...newRunForm, prompt: e.target.value })}
                  placeholder="ej: Modifica el banner principal para que sea responsive y añade estilos de animación..."
                  rows={4}
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowNewRunModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Proponer Run</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
