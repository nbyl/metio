import { Server, Plus } from 'lucide-react';
import { Card, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { cn } from '../../lib/utils';

export interface EmptyStateProps {
  onCreateServer?: () => void;
  className?: string;
}

export function EmptyState({ onCreateServer, className }: EmptyStateProps) {
  return (
    <Card className={cn(className)}>
      <CardContent>
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Server className="h-16 w-16 text-slate-600 mb-4" />
          <h3 className="text-xl font-semibold text-white mb-2">
            No Servers Yet
          </h3>
          <p className="text-slate-400 max-w-md mb-6">
            Get started by creating your first Minecraft server. You can
            configure the region, machine type, and version during setup.
          </p>
          {onCreateServer && (
            <Button variant="default" onClick={onCreateServer}>
              <Plus className="h-4 w-4" />
              Create Server
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
