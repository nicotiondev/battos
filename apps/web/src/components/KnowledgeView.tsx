'use client';

import React, { useCallback, useState, useEffect } from 'react';
import { ApiError, api, apiClient } from '../lib/api';
import { MemoryObservation, MemoryStats, Project } from '../lib/types';
import { 
  Database, Search, Save, BookOpen, FileText,
  Plus, Calendar, Eye, RefreshCw, X, Layers, AlertTriangle
} from 'lucide-react';

interface KnowledgeWorkspace {
  id: string;
  projectId: string;
  name: string;
}

interface JournalEntry {
  id: string;
  projectId: string;
  title: string;
  content: string;
  journalDate?: string;
  createdAt?: string;
}

interface ArtifactEntry {
  id: string;
  projectId: string;
  name: string;
  kind: string;
  content?: string;
  managedPath?: string;
}

type MemoryScope = 'project' | 'personal';
type ArtifactKind = 'markdown' | 'image' | 'link' | 'diff' | 'build_report';

interface MemoryRecentResponse {
  items?: MemoryObservation[];
}

interface MemorySearchResponse {
  results?: MemoryObservation[];
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

export default function KnowledgeView() {
  const [activeTab, setActiveTab] = useState<'memory' | 'journals' | 'artifacts'>('memory');
  const [projects, setProjects] = useState<Project[]>([]);
  const [workspaces, setWorkspaces] = useState<KnowledgeWorkspace[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');

  // 1. Memory Core State
  const [memObservations, setMemObservations] = useState<MemoryObservation[]>([]);
  const [memStats, setMemStats] = useState<MemoryStats | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [newObs, setNewObs] = useState<{ topicKey: string; content: string; scope: MemoryScope }>({ topicKey: '', content: '', scope: 'project' });
  const [showObsModal, setShowObsModal] = useState(false);

  // 2. Journals State
  const [journals, setJournals] = useState<JournalEntry[]>([]);
  const [newJournal, setNewJournal] = useState({ title: '', content: '' });
  const [showJournalModal, setShowJournalModal] = useState(false);

  // 3. Artifacts State
  const [artifacts, setArtifacts] = useState<ArtifactEntry[]>([]);
  const [newArtifact, setNewArtifact] = useState<{ name: string; kind: ArtifactKind; content: string }>({ name: '', kind: 'markdown', content: '' });
  const [showArtifactModal, setShowArtifactModal] = useState(false);

  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState('');
  const [dbUnavailable, setDbUnavailable] = useState(false);

  const fetchCommonData = useCallback(async () => {
    try {
      setDbUnavailable(false);
      const projs = await api.get<Project[]>('/projects');
      const spaces = await api.get<KnowledgeWorkspace[]>('/knowledge/workspaces');
      setProjects(projs);
      setWorkspaces(Array.isArray(spaces) ? spaces : []);
      if (projs.length > 0 && !selectedProjectId) {
        setSelectedProjectId(projs[0].id);
      }
    } catch (err) {
      console.error(err);
      setProjects([]);
      setWorkspaces([]);
      setSelectedProjectId('');
      setDbUnavailable(true);
    }
  }, [selectedProjectId]);

  const fetchMemory = useCallback(async () => {
    try {
      setLoading(true);
      const [recentRes, stats] = await Promise.all([
        api.get<MemoryRecentResponse>('/memory/recent').catch(() => ({ items: [] })),
        api.get<MemoryStats>('/memory/stats').catch(() => ({ totalItems: 0, itemsLast24h: 0 }))
      ]);
      const items = recentRes && Array.isArray(recentRes.items) ? recentRes.items : [];
      setMemObservations(items);
      setMemStats(stats);
    } catch (err) {
      console.error(err);
      setMemObservations([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchJournals = useCallback(async () => {
    if (!selectedProjectId) {
      setJournals([]);
      return;
    }
    try {
      setLoading(true);
      const items = await api.get<JournalEntry[]>(`/journals?project_id=${selectedProjectId}`);
      setJournals(Array.isArray(items) ? items : []);
    } catch (err) {
      console.error(err);
      setJournals([]);
      setDbUnavailable(err instanceof ApiError && err.status === 503);
    } finally {
      setLoading(false);
    }
  }, [selectedProjectId]);

  const fetchArtifacts = useCallback(async () => {
    if (!selectedProjectId) {
      setArtifacts([]);
      return;
    }
    try {
      setLoading(true);
      const items = await api.get<ArtifactEntry[]>(`/artifacts?project_id=${selectedProjectId}`);
      setArtifacts(Array.isArray(items) ? items : []);
    } catch (err) {
      console.error(err);
      setArtifacts([]);
      setDbUnavailable(err instanceof ApiError && err.status === 503);
    } finally {
      setLoading(false);
    }
  }, [selectedProjectId]);

  useEffect(() => {
    fetchCommonData();
  }, [fetchCommonData]);

  useEffect(() => {
    if (activeTab === 'memory') fetchMemory();
    if (activeTab === 'journals') fetchJournals();
    if (activeTab === 'artifacts') fetchArtifacts();
  }, [activeTab, selectedProjectId, fetchMemory, fetchJournals, fetchArtifacts]);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!searchQuery.trim()) {
      fetchMemory();
      return;
    }
    try {
      setLoading(true);
      const res = await apiClient.searchMemory({
        query: searchQuery,
        filter: selectedProjectId ? { projectId: selectedProjectId } : undefined
      }) as MemorySearchResponse;
      const results = res && Array.isArray(res.results) ? res.results : [];
      setMemObservations(results);
    } catch (err) {
      console.error(err);
      setMemObservations([]);
    } finally {
      setLoading(false);
    }
  };

  const ensureWorkspace = async (projectId: string): Promise<string> => {
    const existing = workspaces.find(w => w.projectId === projectId);
    if (existing?.id) {
      return existing.id;
    }
    const project = projects.find(p => p.id === projectId);
    const created = await apiClient.createKnowledgeWorkspace({
      projectId,
      name: `${project?.name || projectId} Knowledge`
    }) as KnowledgeWorkspace;
    setWorkspaces(prev => [...prev, created]);
    return created.id;
  };

  const handleSaveObservation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newObs.topicKey || !newObs.content) return;
    try {
      await apiClient.saveMemoryObservation({
        type: 'manual',
        title: newObs.topicKey,
        topicKey: newObs.topicKey,
        content: newObs.content,
        scope: newObs.scope,
        projectId: selectedProjectId || undefined
      });
      setShowObsModal(false);
      setNewObs({ topicKey: '', content: '', scope: 'project' });
      fetchMemory();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al guardar observación"));
    }
  };

  const handleSaveJournal = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newJournal.title || !newJournal.content || !selectedProjectId) return;
    try {
      const workspaceId = await ensureWorkspace(selectedProjectId);
      await apiClient.createJournal({
        workspaceId,
        projectId: selectedProjectId,
        title: newJournal.title,
        content: newJournal.content
      });
      setShowJournalModal(false);
      setNewJournal({ title: '', content: '' });
      fetchJournals();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al crear journal"));
    }
  };

  const handleSaveArtifact = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newArtifact.name || !newArtifact.content || !selectedProjectId) return;
    try {
      await apiClient.createArtifact({
        projectId: selectedProjectId,
        name: newArtifact.name,
        kind: newArtifact.kind,
        content: newArtifact.content
      });
      setShowArtifactModal(false);
      setNewArtifact({ name: '', kind: 'markdown', content: '' });
      fetchArtifacts();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al registrar artefacto"));
    }
  };

  return (
    <div className="space-y-6">
      {dbUnavailable && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-100">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex gap-3">
              <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
              <div>
                <p className="font-bold text-amber-200">Knowledge Center parcialmente disponible</p>
                <p className="mt-1 text-amber-100/80">
                  Memory Core, workspaces, bitacoras y artefactos viven en la base SQLite local. Esta seccion queda pausada hasta recuperar la DB.
                </p>
              </div>
            </div>
            <button
              onClick={fetchCommonData}
              className="inline-flex items-center justify-center gap-1.5 rounded border border-amber-400/30 bg-black/20 px-3 py-1.5 font-semibold text-amber-100 hover:bg-black/30"
            >
              <RefreshCw size={12} /> Reintentar DB
            </button>
          </div>
        </div>
      )}

      {/* 1. Selector de Pestañas y Filtro de Proyecto */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-gray-800 pb-3">
        <div className="flex gap-2">
          <button 
            onClick={() => setActiveTab('memory')}
            className={`px-4 py-2 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-all ${
              activeTab === 'memory' 
                ? 'bg-primary text-primary-foreground font-bold shadow' 
                : 'bg-transparent text-muted-foreground hover:text-white'
            }`}
          >
            <Database size={14} /> Memory Core (SQLite)
          </button>
          <button 
            onClick={() => setActiveTab('journals')}
            className={`px-4 py-2 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-all ${
              activeTab === 'journals' 
                ? 'bg-primary text-primary-foreground font-bold shadow' 
                : 'bg-transparent text-muted-foreground hover:text-white'
            }`}
          >
            <BookOpen size={14} /> Bitácora (Journals)
          </button>
          <button 
            onClick={() => setActiveTab('artifacts')}
            className={`px-4 py-2 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-all ${
              activeTab === 'artifacts' 
                ? 'bg-primary text-primary-foreground font-bold shadow' 
                : 'bg-transparent text-muted-foreground hover:text-white'
            }`}
          >
            <FileText size={14} /> Entregables (Artifacts)
          </button>
        </div>

        <div className="flex items-center gap-2">
          <Layers size={14} className="text-muted-foreground" />
          <select 
            value={selectedProjectId} 
            onChange={(e) => setSelectedProjectId(e.target.value)}
            className="bg-gray-950 border border-gray-800 text-xs text-white rounded px-2.5 py-1 focus:outline-none focus:border-primary"
            disabled={dbUnavailable}
          >
            <option value="">Filtrar: Global / Inbox</option>
            {projects.map(p => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        </div>
      </div>

      {errorMsg && (
        <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-lg flex justify-between items-center animate-shake">
          <span>{errorMsg}</span>
          <button onClick={() => setErrorMsg('')} className="hover:text-white"><X size={14} /></button>
        </div>
      )}

      {/* 2. CONTENIDO PRINCIPAL SEGÚN PESTAÑA */}
      {loading ? (
        <div className="flex h-[40vh] items-center justify-center">
          <RefreshCw className="animate-spin text-primary" size={32} />
        </div>
      ) : (
        <>
          {/* SECCIÓN: MEMORY CORE */}
          {activeTab === 'memory' && (
            <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
              {/* Buscador y listado */}
              <div className="lg:col-span-3 space-y-4">
                <form onSubmit={handleSearch} className="flex gap-2">
                  <div className="relative flex-1">
                    <Search className="absolute left-3 top-2.5 text-muted-foreground" size={14} />
                    <input 
                      type="text" 
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder="Buscar observaciones de memoria (ej: configs, bugs, decisiones)..."
                      className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded pl-9 p-2.5 focus:outline-none focus:border-primary"
                    />
                  </div>
                  <button type="submit" className="px-4 py-2 text-xs font-semibold rounded bg-indigo-600 hover:bg-indigo-700 text-white">
                    Buscar
                  </button>
                </form>

                <div className="space-y-3">
                  {Array.isArray(memObservations) && memObservations.map(obs => (
                    <div key={obs.id} className="p-4 bg-gray-900/40 border border-gray-800/80 rounded-xl space-y-2">
                      <div className="flex justify-between items-center">
                        <span className="px-2 py-0.5 rounded text-[9px] bg-primary/10 text-primary border border-primary/20 font-semibold font-mono">
                          {obs.topicKey}
                        </span>
                        <span className="text-[9px] text-muted-foreground">
                          {obs.createdAt ? new Date(obs.createdAt).toLocaleDateString() : 'N/A'}
                        </span>
                      </div>
                      <p className="text-xs text-gray-200 whitespace-pre-wrap">{obs.content}</p>
                      <div className="flex items-center gap-1.5 text-[9px] text-muted-foreground pt-1 border-t border-gray-800/40 mt-2">
                        <span className="capitalize">Alcance: {obs.scope}</span>
                      </div>
                    </div>
                  ))}
                  {(!Array.isArray(memObservations) || memObservations.length === 0) && (
                    <div className="flex h-36 items-center justify-center text-xs text-muted-foreground italic bg-gray-900/10 border border-gray-800/30 rounded-xl">
                      Sin observaciones de memoria guardadas en esta sección.
                    </div>
                  )}
                </div>
              </div>

              {/* Lado derecho: estadísticas y guardado manual */}
              <div className="space-y-4">
                <div className="glass-panel p-4 rounded-xl border border-gray-800 space-y-4">
                  <h4 className="text-xs font-bold text-white flex items-center gap-1.5">
                    <Database size={14} className="text-primary" /> Estadísticas del Core
                  </h4>
                  <div className="space-y-2 text-xs">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Total observaciones:</span>
                      <span className="font-bold text-white">{memStats?.totalItems || 0}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Últimas 24 horas:</span>
                      <span className="font-bold text-white">{memStats?.itemsLast24h || 0}</span>
                    </div>
                  </div>
                  <button 
                    onClick={() => setShowObsModal(true)}
                    className="w-full py-1.5 px-3 text-xs font-semibold rounded bg-primary text-primary-foreground hover:bg-yellow-400 flex items-center justify-center gap-1.5"
                  >
                    <Save size={14} /> Guardar Memoria
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* SECCIÓN: BITÁCORA (JOURNALS) */}
          {activeTab === 'journals' && (
            <div className="space-y-4">
              <div className="flex justify-between items-center">
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <BookOpen size={16} className="text-primary" /> Entradas Diarias de Bitácora
                </h3>
                <button 
                  onClick={() => setShowJournalModal(true)}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded bg-primary text-primary-foreground hover:bg-yellow-400 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={dbUnavailable || !selectedProjectId}
                >
                  <Plus size={14} /> Escribir Bitácora
                </button>
              </div>

              {!selectedProjectId ? (
                <div className="flex h-36 items-center justify-center text-xs text-muted-foreground italic bg-gray-900/10 border border-gray-800/30 rounded-xl">
                  Por favor selecciona un proyecto en la parte superior derecha para ver las bitácoras.
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {Array.isArray(journals) && journals.filter(j => j.projectId === selectedProjectId).map(j => (
                    <div key={j.id} className="p-4 bg-gray-900/40 border border-gray-800/80 rounded-xl space-y-3">
                      <div className="flex justify-between items-center">
                        <h4 className="text-xs font-bold text-white">{j.title}</h4>
                        <span className="text-[9px] text-muted-foreground flex items-center gap-1">
                          <Calendar size={10} /> {j.journalDate || (j.createdAt ? new Date(j.createdAt).toLocaleDateString() : 'N/A')}
                        </span>
                      </div>
                      <p className="text-xs text-gray-300 whitespace-pre-wrap line-clamp-4">{j.content}</p>
                      <div className="flex items-center gap-1.5 text-[9px] text-muted-foreground pt-1 border-t border-gray-800/40">
                        <span>Proyecto: {j.projectId}</span>
                      </div>
                    </div>
                  ))}
                  {(!Array.isArray(journals) || journals.filter(j => j.projectId === selectedProjectId).length === 0) && (
                    <div className="md:col-span-2 flex h-36 items-center justify-center text-xs text-muted-foreground italic bg-gray-900/10 border border-gray-800/30 rounded-xl">
                      No se han registrado bitácoras para este proyecto.
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* SECCIÓN: ARTEFACTOS (ARTIFACTS) */}
          {activeTab === 'artifacts' && (
            <div className="space-y-4">
              <div className="flex justify-between items-center">
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <FileText size={16} className="text-primary" /> Catálogo de Artefactos de Referencia
                </h3>
                <button 
                  onClick={() => setShowArtifactModal(true)}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded bg-primary text-primary-foreground hover:bg-yellow-400 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={dbUnavailable || !selectedProjectId}
                >
                  <Plus size={14} /> Registrar Artefacto
                </button>
              </div>

              {!selectedProjectId ? (
                <div className="flex h-36 items-center justify-center text-xs text-muted-foreground italic bg-gray-900/10 border border-gray-800/30 rounded-xl">
                  Por favor selecciona un proyecto en la parte superior derecha para ver los entregables.
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  {Array.isArray(artifacts) && artifacts.filter(a => a.projectId === selectedProjectId).map(a => (
                    <div key={a.id} className="p-4 bg-gray-900/40 border border-gray-800/80 rounded-xl flex flex-col justify-between h-36">
                      <div>
                        <div className="flex justify-between items-start gap-1">
                          <h4 className="text-xs font-bold text-white truncate w-36">{a.name}</h4>
                          <span className="px-1.5 py-0.2 rounded text-[8px] bg-indigo-500/10 text-indigo-300 uppercase border border-indigo-500/20">
                            {a.kind}
                          </span>
                        </div>
                        {a.content && <p className="text-[10px] text-muted-foreground line-clamp-3 mt-2">{a.content}</p>}
                      </div>
                      <div className="flex justify-between items-center pt-2 border-t border-gray-800/40 text-[9px] text-muted-foreground">
                        <span>Proyecto: {a.projectId}</span>
                        {a.managedPath && (
                          <span className="text-primary flex items-center gap-0.5">
                            <Eye size={10} /> Local
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                  {(!Array.isArray(artifacts) || artifacts.filter(a => a.projectId === selectedProjectId).length === 0) && (
                    <div className="md:col-span-3 flex h-36 items-center justify-center text-xs text-muted-foreground italic bg-gray-900/10 border border-gray-800/30 rounded-xl">
                      Ningún artefacto registrado en esta sección.
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </>
      )}

      {/* 4. MODALES */}
      {/* Modal Memory Observation */}
      {showObsModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Database size={16} className="text-primary" /> Guardar Observación en Memoria
              </h3>
              <button onClick={() => setShowObsModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleSaveObservation} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Clave de Tema (Topic Key)</label>
                <input 
                  type="text" 
                  value={newObs.topicKey} 
                  onChange={(e) => setNewObs({ ...newObs, topicKey: e.target.value.toLowerCase().replace(/[^a-z0-9-/]/g, '') })}
                  placeholder="ej: nico/work-style"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Alcance (Scope)</label>
                <select 
                  value={newObs.scope} 
                  onChange={(e) => setNewObs({ ...newObs, scope: e.target.value as MemoryScope })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="project">Proyecto (scope=project)</option>
                  <option value="personal">Preferencias Personales (scope=personal)</option>
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Contenido de la Observación</label>
                <textarea 
                  value={newObs.content} 
                  onChange={(e) => setNewObs({ ...newObs, content: e.target.value })}
                  placeholder="Escribe el aprendizaje o decisión a registrar..."
                  rows={4}
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowObsModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Guardar</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Journal */}
      {showJournalModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <BookOpen size={16} className="text-primary" /> Escribir Entrada de Bitácora
              </h3>
              <button onClick={() => setShowJournalModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleSaveJournal} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Título de la Entrada</label>
                <input 
                  type="text" 
                  value={newJournal.title} 
                  onChange={(e) => setNewJournal({ ...newJournal, title: e.target.value })}
                  placeholder="ej: Bitácora de Onboarding"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Contenido de la Bitácora</label>
                <textarea 
                  value={newJournal.content} 
                  onChange={(e) => setNewJournal({ ...newJournal, content: e.target.value })}
                  placeholder="Describe las actividades diarias o progresos..."
                  rows={6}
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowJournalModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Crear Entrada</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Artifact */}
      {showArtifactModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <FileText size={16} className="text-primary" /> Registrar Artefacto
              </h3>
              <button onClick={() => setShowArtifactModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleSaveArtifact} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Nombre</label>
                  <input 
                    type="text" 
                    value={newArtifact.name} 
                    onChange={(e) => setNewArtifact({ ...newArtifact, name: e.target.value })}
                    placeholder="ej: brief-proyecto.md"
                    required
                    className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Tipo (Kind)</label>
                  <select 
                    value={newArtifact.kind} 
                    onChange={(e) => setNewArtifact({ ...newArtifact, kind: e.target.value as ArtifactKind })}
                    className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                  >
                    <option value="markdown">Documento Markdown</option>
                    <option value="image">Imagen</option>
                    <option value="link">Enlace Web</option>
                    <option value="diff">Diff de codigo</option>
                    <option value="build_report">Reporte de build</option>
                  </select>
                </div>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Contenido / Enlace</label>
                <textarea 
                  value={newArtifact.content} 
                  onChange={(e) => setNewArtifact({ ...newArtifact, content: e.target.value })}
                  placeholder="Ingresa la especificación, texto o URL del artefacto..."
                  rows={5}
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowArtifactModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Registrar</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
