import { useState } from 'react';
import { X, AlertTriangle } from 'lucide-react';
import { Button } from '../ui/Button';
import { cn } from '../../lib/utils';

export interface DestroyModalProps {
  open: boolean;
  serverName: string;
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function DestroyModal({
  open,
  serverName,
  onClose,
  onConfirm,
  isPending,
}: DestroyModalProps) {
  const [confirmText, setConfirmText] = useState('');

  if (!open) return null;

  const isConfirmed = confirmText === serverName;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div
        className={cn(
          'bg-slate-800 rounded-xl border border-slate-700 shadow-xl',
          'w-full max-w-md mx-4'
        )}
      >
        <div className="flex items-center justify-between px-6 pt-6 pb-4">
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-red-400" />
            Destroy Server
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="text-slate-400 hover:text-white transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="px-6 pb-6 space-y-4">
          <div className="space-y-2">
            <p className="text-sm text-slate-300">
              This action will permanently destroy the server{' '}
              <strong className="text-white">{serverName}</strong> and all its
              data. This cannot be undone.
            </p>
            <p className="text-sm text-red-400 font-medium">
              Type the server name to confirm:
            </p>
            <input
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={serverName}
              className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-red-500 focus:border-transparent"
            />
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={isPending}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="danger"
              disabled={!isConfirmed || isPending}
              loading={isPending}
              onClick={onConfirm}
            >
              Destroy Server
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
