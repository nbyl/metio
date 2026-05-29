import { useState } from 'react';
import { ChevronDown, ChevronRight, Settings2, Globe, Cpu, HardDrive, Calendar } from 'lucide-react';
import type { ServerConfig } from '../../types/server';
import { Card, CardContent } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { Separator } from '../ui/Separator';
import { cn } from '../../lib/utils';

export interface ServerConfigPanelProps {
  config: ServerConfig;
  infrahVersion: number;
  outdated: boolean;
  className?: string;
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return iso;
  }
}

export function ServerConfigPanel({
  config,
  infrahVersion,
  outdated,
  className,
}: ServerConfigPanelProps) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <Card className={cn(className)}>
      <CardContent>
        <button
          type="button"
          className="collapsible-trigger"
          onClick={() => setIsOpen(!isOpen)}
        >
          <span className="flex items-center gap-2 text-sm font-medium text-slate-300">
            <Settings2 className="h-4 w-4" />
            Server Configuration
          </span>
          <span className="flex items-center gap-2">
            {outdated && (
              <Badge variant="transitioning">Update Available</Badge>
            )}
            {isOpen ? (
              <ChevronDown className="h-4 w-4 text-slate-400" />
            ) : (
              <ChevronRight className="h-4 w-4 text-slate-400" />
            )}
          </span>
        </button>

        <div className="collapsible-content" data-state={isOpen ? 'open' : 'closed'}>
          {isOpen && (
            <div className="space-y-3 pt-3">
              <Separator />

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                <div className="stat">
                  <span className="stat-label">
                    <Globe className="h-3.5 w-3.5" />
                    Name
                  </span>
                  <span className="stat-value">{config.name}</span>
                </div>

                <div className="stat">
                  <span className="stat-label">
                    <Globe className="h-3.5 w-3.5" />
                    Location
                  </span>
                  <span className="stat-value">
                    {config.region}/{config.zone}
                  </span>
                </div>

                <div className="stat">
                  <span className="stat-label">
                    <Cpu className="h-3.5 w-3.5" />
                    Machine Type
                  </span>
                  <span className="stat-value">{config.machineType}</span>
                </div>

                <div className="stat">
                  <span className="stat-label">
                    <HardDrive className="h-3.5 w-3.5" />
                    Disk Size
                  </span>
                  <span className="stat-value">{config.diskSizeGB} GB</span>
                </div>

                <div className="stat">
                  <span className="stat-label">
                    Minecraft Version
                  </span>
                  <span className="stat-value">{config.minecraftVersion}</span>
                </div>

                <div className="stat">
                  <span className="stat-label">
                    Infra Version
                  </span>
                  <span className="stat-value">v{infrahVersion}</span>
                </div>
              </div>

              <Separator />

              <div className="text-xs text-slate-500 space-y-1">
                <div className="flex items-center gap-1">
                  <Calendar className="h-3 w-3" />
                  Created: {formatDate(config.createdAt)}
                </div>
                <div className="flex items-center gap-1">
                  <Calendar className="h-3 w-3" />
                  Updated: {formatDate(config.updatedAt)}
                </div>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
