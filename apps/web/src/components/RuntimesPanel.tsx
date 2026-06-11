'use client';

import React from 'react';
import { AgentRuntime } from '../lib/types';
import { Server, CheckCircle, XCircle, AlertCircle, Clock, Shield } from 'lucide-react';

interface RuntimesPanelProps {
  runtimes: AgentRuntime[];
}

function getStatusIcon(status: string) {
  switch (status) {
    case 'configured':
      return <CheckCircle size={12} className="text-emerald-400" />;
    case 'detected':
      return <CheckCircle size={12} className="text-blue-400" />;
    case 'unavailable':
      return <XCircle size={12} className="text-red-400" />;
    case 'blocked':
      return <AlertCircle size={12} className="text-amber-400" />;
    case 'disabled':
      return <XCircle size={12} className="text-gray-500" />;
    default:
      return <AlertCircle size={12} className="text-gray-400" />;
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case 'configured':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold">
          Configurado
        </span>
      );
    case 'detected':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-blue-500/10 text-blue-400 border border-blue-500/20 font-semibold">
          Detectado
        </span>
      );
    case 'unavailable':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-red-500/10 text-red-400 border border-red-500/20 font-semibold">
          No disponible
        </span>
      );
    case 'blocked':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-400 border border-amber-500/20 font-semibold">
          Bloqueado
        </span>
      );
    case 'disabled':
      return (
        <span className="px-1.5 py-0.5 rounded text-[10px] bg-gray-500/10 text-gray-400 border border-gray-500/20 font-semibold">
          Deshabilitado
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

export default function RuntimesPanel({ runtimes }: RuntimesPanelProps) {
  if (runtimes.length === 0) {
    return (
      <div className="glass-panel rounded-xl border border-gray-800 p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-xs font-bold text-white flex items-center gap-2">
            <Server size={14} className="text-primary" /> Runtimes Activos
          </h3>
        </div>
        <p className="text-[10px] text-muted-foreground italic text-center py-3">
          No hay runtime adapters configurados.
        </p>
      </div>
    );
  }

  return (
    <div className="glass-panel rounded-xl border border-gray-800 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-bold text-white flex items-center gap-2">
          <Server size={14} className="text-primary" /> Runtimes Activos
        </h3>
        <span className="text-[10px] text-muted-foreground">{runtimes.length} adaptadores</span>
      </div>

      <div className="space-y-2">
        {runtimes.map((rt) => (
          <div
            key={rt.id}
            className="flex items-start justify-between gap-2 p-2.5 rounded-lg bg-gray-900/50 border border-gray-800/60"
          >
            <div className="flex items-start gap-2 min-w-0">
              <div className="mt-0.5 shrink-0">{getStatusIcon(rt.status)}</div>
              <div className="min-w-0">
                <p className="text-[11px] font-semibold text-white truncate">{rt.name}</p>
                <div className="flex flex-wrap items-center gap-1.5 mt-0.5">
                  {rt.command && (
                    <span className="font-mono text-[9px] text-muted-foreground bg-gray-950 px-1 py-0.5 rounded">
                      {rt.command}
                    </span>
                  )}
                  {rt.version && (
                    <span className="text-[9px] text-muted-foreground">v{rt.version}</span>
                  )}
                  {rt.requiresAuth && (
                    <span className="flex items-center gap-0.5 text-[9px] text-amber-400">
                      <Shield size={8} /> auth
                    </span>
                  )}
                  {rt.approvedForExecution && (
                    <span className="flex items-center gap-0.5 text-[9px] text-emerald-400">
                      <CheckCircle size={8} /> aprobado
                    </span>
                  )}
                </div>
                {rt.lastDetectedAt && (
                  <div className="flex items-center gap-0.5 mt-0.5">
                    <Clock size={8} className="text-muted-foreground" />
                    <span className="text-[9px] text-muted-foreground">
                      {new Date(rt.lastDetectedAt).toLocaleString('es', {
                        month: 'short', day: 'numeric',
                        hour: '2-digit', minute: '2-digit',
                      })}
                    </span>
                  </div>
                )}
              </div>
            </div>
            <div className="shrink-0">{getStatusBadge(rt.status)}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
