'use client';

import React, { useCallback, useState, useEffect } from 'react';
import { ApiError, apiClient } from '../lib/api';
import { Project, Goal, Task, Agent } from '../lib/types';
import { 
  Plus, CheckSquare, Layers, ArrowRight, ArrowLeft,
  User, X, Edit, FolderPlus, Compass, AlertTriangle, RefreshCw
} from 'lucide-react';

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

type TaskStatus = 'backlog' | 'ready' | 'in_progress' | 'review' | 'done' | 'cancelled';

export default function WorkboardView() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [goals, setGoals] = useState<Goal[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');
  
  // Modales
  const [showProjectModal, setShowProjectModal] = useState(false);
  const [showGoalModal, setShowGoalModal] = useState(false);
  const [showTaskModal, setShowTaskModal] = useState(false);
  const [showEditTaskModal, setShowEditTaskModal] = useState(false);
  
  // Formularios
  const [projForm, setProjForm] = useState({ id: '', name: '', description: '' });
  const [goalForm, setGoalForm] = useState({ title: '', description: '', projectId: '' });
  const [taskForm, setTaskForm] = useState({ title: '', description: '', projectId: '', goalId: '', assignedAgentId: '' });
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [editTaskForm, setEditTaskForm] = useState<{ title: string; description: string; projectId: string; goalId: string; assignedAgentId: string; status: TaskStatus }>({ title: '', description: '', projectId: '', goalId: '', assignedAgentId: '', status: 'backlog' });
  
  const [errorMsg, setErrorMsg] = useState('');
  const [dataUnavailable, setDataUnavailable] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      setErrorMsg('');
      setDataUnavailable(false);
      const [projs, ags, gls, tks] = await Promise.all([
        apiClient.listProjects(),
        apiClient.listAgents(),
        apiClient.listGoals(),
        apiClient.listTasks()
      ]);
      setProjects(projs as Project[]);
      setAgents(ags as Agent[]);
      setGoals(gls as Goal[]);
      setTasks(tks as Task[]);
      if (projs.length > 0 && !selectedProjectId) {
        setSelectedProjectId(projs[0].id);
      }
    } catch (err) {
      console.error("Error fetching workboard data", err);
      setProjects([]);
      setAgents([]);
      setGoals([]);
      setTasks([]);
      setDataUnavailable(true);
      if (err instanceof ApiError && err.status === 503) {
        setErrorMsg("Work Board no esta disponible porque la base de datos esta desconectada.");
      } else {
        setErrorMsg("No se pudo cargar Work Board. Revisa la API y vuelve a intentar.");
      }
    }
  }, [selectedProjectId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleCreateProject = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!projForm.id || !projForm.name) return;
    try {
      await apiClient.createProject({
        slug: projForm.id,
        name: projForm.name,
        description: projForm.description,
        status: 'active'
      });
      setShowProjectModal(false);
      setProjForm({ id: '', name: '', description: '' });
      fetchData();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al crear proyecto"));
    }
  };

  const handleCreateGoal = async (e: React.FormEvent) => {
    e.preventDefault();
    const pid = goalForm.projectId || selectedProjectId;
    if (!goalForm.title || !pid) return;
    try {
      await apiClient.createGoal({
        projectId: pid,
        title: goalForm.title,
        description: goalForm.description,
        status: 'active'
      });
      setShowGoalModal(false);
      setGoalForm({ title: '', description: '', projectId: '' });
      fetchData();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al crear objetivo"));
    }
  };

  const handleCreateTask = async (e: React.FormEvent) => {
    e.preventDefault();
    const pid = taskForm.projectId || selectedProjectId;
    if (!taskForm.title || !pid) return;
    try {
      await apiClient.createTask({
        projectId: pid,
        goalId: taskForm.goalId || undefined,
        title: taskForm.title,
        description: taskForm.description,
        assignedAgentId: taskForm.assignedAgentId || undefined,
        boardPosition: 0,
        status: 'backlog'
      });
      setShowTaskModal(false);
      setTaskForm({ title: '', description: '', projectId: '', goalId: '', assignedAgentId: '' });
      fetchData();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al crear tarea"));
    }
  };

  const moveTaskStatus = async (task: Task, nextStatus: TaskStatus) => {
    try {
      await apiClient.updateTask(task.id, {
        status: nextStatus
      });
      fetchData();
    } catch (err) {
      console.error("Error al mover tarea", err);
    }
  };

  const handleEditTaskClick = (task: Task) => {
    setSelectedTask(task);
    setEditTaskForm({
      title: task.title,
      description: task.description || '',
      projectId: task.projectId,
      goalId: task.goalId || '',
      assignedAgentId: task.assignedAgentId || '',
      status: task.status as TaskStatus
    });
    setShowEditTaskModal(true);
  };

  const handleUpdateTask = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedTask || !editTaskForm.title) return;
    try {
      await apiClient.updateTask(selectedTask.id, {
        title: editTaskForm.title,
        description: editTaskForm.description,
        projectId: editTaskForm.projectId,
        goalId: editTaskForm.goalId || undefined,
        assignedAgentId: editTaskForm.assignedAgentId || undefined,
        status: editTaskForm.status
      });
      setShowEditTaskModal(false);
      setSelectedTask(null);
      fetchData();
    } catch (err: unknown) {
      setErrorMsg(errorMessage(err, "Error al actualizar la tarea"));
    }
  };

  // Filtrar tareas por proyecto seleccionado
  const filteredTasks = tasks.filter(t => t.projectId === selectedProjectId);
  const filteredGoals = goals.filter(g => g.projectId === selectedProjectId);
  const uniqueProjects = Array.from(new Map(projects.map(p => [p.id, p])).values());

  const todoTasks = filteredTasks.filter(t => t.status === 'backlog' || t.status === 'ready');
  const inProgressTasks = filteredTasks.filter(t => t.status === 'in_progress');
  const doneTasks = filteredTasks.filter(t => t.status === 'done');

  return (
    <div className="space-y-6">
      {dataUnavailable && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-100">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex gap-3">
              <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
              <div>
                <p className="font-bold text-amber-200">Datos de Work Board no disponibles</p>
                <p className="mt-1 text-amber-100/80">
                  Proyectos, objetivos, tareas y agentes dependen de Postgres. BattOS sigue vivo, pero esta pantalla queda en modo diagnostico hasta recuperar la DB.
                </p>
              </div>
            </div>
            <button
              onClick={fetchData}
              className="inline-flex items-center justify-center gap-1.5 rounded border border-amber-400/30 bg-black/20 px-3 py-1.5 font-semibold text-amber-100 hover:bg-black/30"
            >
              <RefreshCw size={12} /> Reintentar
            </button>
          </div>
        </div>
      )}

      {/* Selector de Proyecto y Botones de Creación */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-gray-900/40 p-4 rounded-xl border border-gray-800/80">
        <div className="flex items-center gap-3">
          <Layers size={18} className="text-primary" />
          <span className="text-xs font-semibold text-muted-foreground uppercase">Proyecto Activo:</span>
          <select 
            value={selectedProjectId} 
            onChange={(e) => setSelectedProjectId(e.target.value)}
            className="bg-gray-950 border border-gray-800 text-sm text-white rounded px-3 py-1.5 focus:outline-none focus:border-primary"
          >
            {uniqueProjects.map(p => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
            {uniqueProjects.length === 0 && <option value="">(Sin proyectos)</option>}
          </select>
        </div>

        <div className="flex gap-2">
          <button 
            onClick={() => setShowProjectModal(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded bg-gray-900 hover:bg-gray-800 text-gray-200 border border-gray-800"
            disabled={dataUnavailable}
          >
            <Plus size={14} /> Nuevo Proyecto
          </button>
          <button 
            onClick={() => setShowGoalModal(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded bg-gray-900 hover:bg-gray-800 text-gray-200 border border-gray-800"
            disabled={dataUnavailable || !selectedProjectId}
          >
            <Plus size={14} /> Nuevo Goal
          </button>
          <button 
            onClick={() => setShowTaskModal(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded bg-primary text-primary-foreground hover:bg-yellow-400"
            disabled={dataUnavailable || !selectedProjectId}
          >
            <Plus size={14} /> Nueva Tarea
          </button>
        </div>
      </div>

      {errorMsg && (
        <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-lg flex justify-between items-center">
          <span>{errorMsg}</span>
          <button onClick={() => setErrorMsg('')} className="hover:text-white"><X size={14} /></button>
        </div>
      )}

      {/* Tablero Kanban */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Columna: To Do */}
        <div className="glass-panel p-4 rounded-xl space-y-4">
          <div className="flex items-center justify-between border-b border-gray-800 pb-2">
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-blue-500" />
              Por Hacer
            </h3>
            <span className="px-2 py-0.5 rounded text-[10px] bg-gray-900 text-muted-foreground font-semibold">
              {todoTasks.length}
            </span>
          </div>

          <div className="space-y-3 min-h-[40vh]">
            {todoTasks.map(t => (
              <div 
                key={t.id} 
                onClick={() => handleEditTaskClick(t)}
                className="p-3 bg-gray-950/80 border border-gray-800/80 rounded-lg hover:border-primary/50 cursor-pointer transition-all space-y-2"
              >
                <h4 className="text-xs font-bold text-white">{t.title}</h4>
                {t.description && <p className="text-[10px] text-muted-foreground line-clamp-2">{t.description}</p>}
                
                <div className="flex justify-between items-center pt-2">
                  <span className="text-[9px] px-1.5 py-0.5 rounded bg-gray-900 text-gray-400 flex items-center gap-1">
                    <User size={10} /> {t.assignedAgentId || 'Sin asignar'}
                  </span>
                  
                  <button 
                    onClick={(e) => {
                      e.stopPropagation();
                      moveTaskStatus(t, 'in_progress');
                    }}
                    className="p-1 rounded bg-gray-900 hover:bg-gray-800 text-primary"
                    title="Mover a En Progreso"
                  >
                    <ArrowRight size={12} />
                  </button>
                </div>
              </div>
            ))}
            {todoTasks.length === 0 && (
              <div className="flex h-24 items-center justify-center text-[10px] text-muted-foreground italic">
                Sin tareas en esta columna.
              </div>
            )}
          </div>
        </div>

        {/* Columna: In Progress */}
        <div className="glass-panel p-4 rounded-xl space-y-4">
          <div className="flex items-center justify-between border-b border-gray-800 pb-2">
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-amber-500" />
              En Progreso
            </h3>
            <span className="px-2 py-0.5 rounded text-[10px] bg-gray-900 text-muted-foreground font-semibold">
              {inProgressTasks.length}
            </span>
          </div>

          <div className="space-y-3 min-h-[40vh]">
            {inProgressTasks.map(t => (
              <div 
                key={t.id} 
                onClick={() => handleEditTaskClick(t)}
                className="p-3 bg-gray-950/80 border border-gray-800/80 rounded-lg hover:border-primary/50 cursor-pointer transition-all space-y-2"
              >
                <h4 className="text-xs font-bold text-white">{t.title}</h4>
                {t.description && <p className="text-[10px] text-muted-foreground line-clamp-2">{t.description}</p>}
                
                <div className="flex justify-between items-center pt-2">
                  <span className="text-[9px] px-1.5 py-0.5 rounded bg-gray-900 text-gray-400 flex items-center gap-1">
                    <User size={10} /> {t.assignedAgentId || 'Sin asignar'}
                  </span>
                  
                  <div className="flex gap-1">
                    <button 
                      onClick={(e) => {
                        e.stopPropagation();
                        moveTaskStatus(t, 'backlog');
                      }}
                      className="p-1 rounded bg-gray-900 hover:bg-gray-800 text-gray-400"
                      title="Mover a Por Hacer"
                    >
                      <ArrowLeft size={12} />
                    </button>
                    <button 
                      onClick={(e) => {
                        e.stopPropagation();
                        moveTaskStatus(t, 'done');
                      }}
                      className="p-1 rounded bg-gray-900 hover:bg-gray-800 text-emerald-400"
                      title="Marcar como Completada"
                    >
                      <ArrowRight size={12} />
                    </button>
                  </div>
                </div>
              </div>
            ))}
            {inProgressTasks.length === 0 && (
              <div className="flex h-24 items-center justify-center text-[10px] text-muted-foreground italic">
                Sin tareas en progreso.
              </div>
            )}
          </div>
        </div>

        {/* Columna: Done */}
        <div className="glass-panel p-4 rounded-xl space-y-4">
          <div className="flex items-center justify-between border-b border-gray-800 pb-2">
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-emerald-500" />
              Completado
            </h3>
            <span className="px-2 py-0.5 rounded text-[10px] bg-gray-900 text-muted-foreground font-semibold">
              {doneTasks.length}
            </span>
          </div>

          <div className="space-y-3 min-h-[40vh]">
            {doneTasks.map(t => (
              <div 
                key={t.id} 
                onClick={() => handleEditTaskClick(t)}
                className="p-3 bg-gray-950/40 border border-gray-905/80 rounded-lg space-y-2 opacity-70 hover:border-primary/30 cursor-pointer transition-all"
              >
                <h4 className="text-xs font-bold text-gray-400 line-through">{t.title}</h4>
                {t.description && <p className="text-[10px] text-gray-500 line-clamp-2">{t.description}</p>}
                
                <div className="flex justify-between items-center pt-2">
                  <span className="text-[9px] px-1.5 py-0.5 rounded bg-gray-900 text-gray-500 flex items-center gap-1">
                    <User size={10} /> {t.assignedAgentId || 'Sin asignar'}
                  </span>
                  
                  <button 
                    onClick={(e) => {
                      e.stopPropagation();
                      moveTaskStatus(t, 'in_progress');
                    }}
                    className="p-1 rounded bg-gray-900 hover:bg-gray-800 text-gray-400"
                    title="Mover de vuelta a En Progreso"
                  >
                    <ArrowLeft size={12} />
                  </button>
                </div>
              </div>
            ))}
            {doneTasks.length === 0 && (
              <div className="flex h-24 items-center justify-center text-[10px] text-muted-foreground italic">
                Sin tareas completadas.
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 4. MODALES */}
      {/* Modal Proyecto */}
      {showProjectModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <FolderPlus size={16} className="text-primary" /> Crear Nuevo Proyecto
              </h3>
              <button onClick={() => setShowProjectModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleCreateProject} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">ID (Slug único)</label>
                <input 
                  type="text" 
                  value={projForm.id} 
                  onChange={(e) => setProjForm({ ...projForm, id: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '') })}
                  placeholder="ej: clinica-norte"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Nombre</label>
                <input 
                  type="text" 
                  value={projForm.name} 
                  onChange={(e) => setProjForm({ ...projForm, name: e.target.value })}
                  placeholder="ej: Clínica del Norte Landing"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Descripción</label>
                <textarea 
                  value={projForm.description} 
                  onChange={(e) => setProjForm({ ...projForm, description: e.target.value })}
                  placeholder="ej: Desarrollo de landing page moderna..."
                  rows={3}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowProjectModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Crear Proyecto</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Goal */}
      {showGoalModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Compass size={16} className="text-primary" /> Crear Nuevo Goal
              </h3>
              <button onClick={() => setShowGoalModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleCreateGoal} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Título del Goal</label>
                <input 
                  type="text" 
                  value={goalForm.title} 
                  onChange={(e) => setGoalForm({ ...goalForm, title: e.target.value })}
                  placeholder="ej: Completar la maquetación del home"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Descripción</label>
                <textarea 
                  value={goalForm.description} 
                  onChange={(e) => setGoalForm({ ...goalForm, description: e.target.value })}
                  placeholder="ej: Dejar lista la estructura y el CSS responsivo..."
                  rows={3}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowGoalModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Crear Goal</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Tarea */}
      {showTaskModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <CheckSquare size={16} className="text-primary" /> Crear Nueva Tarea
              </h3>
              <button onClick={() => setShowTaskModal(false)} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleCreateTask} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Título de la Tarea</label>
                <input 
                  type="text" 
                  value={taskForm.title} 
                  onChange={(e) => setTaskForm({ ...taskForm, title: e.target.value })}
                  placeholder="ej: Maquetar banner principal"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Objetivo (Goal asociado)</label>
                <select 
                  value={taskForm.goalId} 
                  onChange={(e) => setTaskForm({ ...taskForm, goalId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="">Sin asociar a un Goal</option>
                  {filteredGoals.map(g => (
                    <option key={g.id} value={g.id}>{g.title}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Asignar a Agente</label>
                <select 
                  value={taskForm.assignedAgentId} 
                  onChange={(e) => setTaskForm({ ...taskForm, assignedAgentId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="">Sin asignar (Inbox)</option>
                  {agents.map(a => (
                    <option key={a.id} value={a.id}>{a.name} ({a.role || 'Agent'})</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Descripción de la Tarea</label>
                <textarea 
                  value={taskForm.description} 
                  onChange={(e) => setTaskForm({ ...taskForm, description: e.target.value })}
                  placeholder="ej: Maquetar el banner responsivo con un botón de llamada a la acción y un fondo oscuro..."
                  rows={3}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => setShowTaskModal(false)} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Crear Tarea</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Editar Tarea */}
      {showEditTaskModal && selectedTask && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-md p-6 rounded-xl space-y-4">
            <div className="flex justify-between items-center border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Edit size={16} className="text-primary" /> Editar Tarea
              </h3>
              <button onClick={() => { setShowEditTaskModal(false); setSelectedTask(null); }} className="text-muted-foreground hover:text-white"><X size={16} /></button>
            </div>
            <form onSubmit={handleUpdateTask} className="space-y-4">
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Título de la Tarea</label>
                <input 
                  type="text" 
                  value={editTaskForm.title} 
                  onChange={(e) => setEditTaskForm({ ...editTaskForm, title: e.target.value })}
                  placeholder="ej: Maquetar banner principal"
                  required
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Objetivo (Goal asociado)</label>
                <select 
                  value={editTaskForm.goalId} 
                  onChange={(e) => setEditTaskForm({ ...editTaskForm, goalId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="">Sin asociar a un Goal</option>
                  {filteredGoals.map(g => (
                    <option key={g.id} value={g.id}>{g.title}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Asignar a Agente</label>
                <select 
                  value={editTaskForm.assignedAgentId} 
                  onChange={(e) => setEditTaskForm({ ...editTaskForm, assignedAgentId: e.target.value })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="">Sin asignar (Inbox)</option>
                  {agents.map(a => (
                    <option key={a.id} value={a.id}>{a.name} ({a.role || 'Agent'})</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Estado</label>
                <select 
                  value={editTaskForm.status} 
                  onChange={(e) => setEditTaskForm({ ...editTaskForm, status: e.target.value as TaskStatus })}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                >
                  <option value="backlog">Por Hacer</option>
                  <option value="in_progress">En Progreso</option>
                  <option value="done">Completado</option>
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-muted-foreground uppercase font-semibold mb-1">Descripción de la Tarea</label>
                <textarea 
                  value={editTaskForm.description} 
                  onChange={(e) => setEditTaskForm({ ...editTaskForm, description: e.target.value })}
                  placeholder="ej: Maquetar el banner responsivo..."
                  rows={3}
                  className="w-full bg-gray-950 border border-gray-800 text-xs text-white rounded p-2 focus:outline-none focus:border-primary"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={() => { setShowEditTaskModal(false); setSelectedTask(null); }} className="px-3 py-1.5 text-xs bg-gray-900 hover:bg-gray-800 text-white rounded">Cancelar</button>
                <button type="submit" className="px-3 py-1.5 text-xs bg-primary text-primary-foreground font-semibold rounded">Guardar Cambios</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
