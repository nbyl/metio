import { useState } from 'react';
import { Settings, Trash2 } from 'lucide-react';
import type { ServerConfig, UpdateServerRequest } from '../../types/server';
import { Button } from '../ui/Button';
import { UpdateModal } from './UpdateModal';
import { DestroyModal } from './DestroyModal';
import { cn } from '../../lib/utils';

export interface ServerActionsProps {
  config: ServerConfig;
  onUpdate: (data: UpdateServerRequest) => void;
  onDestroy: () => void;
  isUpdatePending: boolean;
  isDestroyPending: boolean;
  className?: string;
}

export function ServerActions({
  config,
  onUpdate,
  onDestroy,
  isUpdatePending,
  isDestroyPending,
  className,
}: ServerActionsProps) {
  const [showUpdate, setShowUpdate] = useState(false);
  const [showDestroy, setShowDestroy] = useState(false);

  return (
    <>
      <div className={cn('flex gap-3', className)}>
        <Button
          variant="outline"
          onClick={() => setShowUpdate(true)}
          disabled={isDestroyPending}
        >
          <Settings className="h-4 w-4" />
          Update
        </Button>
        <Button
          variant="danger"
          onClick={() => setShowDestroy(true)}
          disabled={isUpdatePending}
        >
          <Trash2 className="h-4 w-4" />
          Destroy
        </Button>
      </div>

      <UpdateModal
        open={showUpdate}
        config={config}
        onClose={() => setShowUpdate(false)}
        onUpdate={(data) => {
          onUpdate(data);
          setShowUpdate(false);
        }}
        isPending={isUpdatePending}
      />

      <DestroyModal
        open={showDestroy}
        serverName={config.name}
        onClose={() => setShowDestroy(false)}
        onConfirm={() => {
          onDestroy();
          setShowDestroy(false);
        }}
        isPending={isDestroyPending}
      />
    </>
  );
}
