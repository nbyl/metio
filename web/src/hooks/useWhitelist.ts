import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  WhitelistResponse,
  WhitelistPlayer,
  AddPlayerRequest,
} from '../types/whitelist';
import type { APIError } from '../types/server';

async function fetchWhitelist(serverId: string): Promise<WhitelistResponse> {
  const response = await fetch(`/api/servers/${serverId}/whitelist`, {
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to fetch whitelist');
  }
  return response.json();
}

async function addPlayer(
  serverId: string,
  username: string
): Promise<WhitelistPlayer> {
  const response = await fetch(`/api/servers/${serverId}/whitelist`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username } as AddPlayerRequest),
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to add player');
  }
  return response.json();
}

async function removePlayer(serverId: string, uuid: string): Promise<void> {
  const response = await fetch(`/api/servers/${serverId}/whitelist/${uuid}`, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to remove player');
  }
}

async function setWhitelistEnabled(
  serverId: string,
  enabled: boolean
): Promise<void> {
  const response = await fetch(`/api/servers/${serverId}/whitelist/enabled`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to update whitelist status');
  }
}

export function useWhitelist(serverId: string) {
  return useQuery({
    queryKey: ['whitelist', serverId],
    queryFn: () => fetchWhitelist(serverId),
    refetchInterval: 10000,
    refetchIntervalInBackground: false,
  });
}

export function useAddPlayer(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (username: string) => addPlayer(serverId, username),
    onSuccess: (newPlayer) => {
      queryClient.setQueryData<WhitelistResponse>(
        ['whitelist', serverId],
        (old) => {
          if (!old) return old;
          return {
            ...old,
            players: [...old.players, newPlayer],
          };
        }
      );
      toast.success(`Added ${newPlayer.username} to whitelist`);
      queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}

export function useRemovePlayer(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (uuid: string) => removePlayer(serverId, uuid),
    onMutate: async (uuid) => {
      await queryClient.cancelQueries({ queryKey: ['whitelist', serverId] });

      const previousWhitelist = queryClient.getQueryData<WhitelistResponse>([
        'whitelist',
        serverId,
      ]);

      queryClient.setQueryData<WhitelistResponse>(
        ['whitelist', serverId],
        (old) => {
          if (!old) return old;
          return {
            ...old,
            players: old.players.filter((p) => p.uuid !== uuid),
          };
        }
      );

      return { previousWhitelist };
    },
    onSuccess: () => {
      toast.success('Player removed from whitelist');
      queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] });
    },
    onError: (error: Error, _uuid, context) => {
      if (context?.previousWhitelist) {
        queryClient.setQueryData(
          ['whitelist', serverId],
          context.previousWhitelist
        );
      }
      toast.error(error.message);
    },
  });
}

export function useToggleWhitelist(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (enabled: boolean) => setWhitelistEnabled(serverId, enabled),
    onMutate: async (enabled) => {
      await queryClient.cancelQueries({ queryKey: ['whitelist', serverId] });

      const previousWhitelist = queryClient.getQueryData<WhitelistResponse>([
        'whitelist',
        serverId,
      ]);

      queryClient.setQueryData<WhitelistResponse>(
        ['whitelist', serverId],
        (old) => {
          if (!old) return old;
          return { ...old, enabled };
        }
      );

      return { previousWhitelist };
    },
    onSuccess: (_data, enabled) => {
      toast.success(`Whitelist ${enabled ? 'enabled' : 'disabled'}`);
      queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] });
      queryClient.invalidateQueries({ queryKey: ['serverStatus', serverId] });
    },
    onError: (error: Error, _enabled, context) => {
      if (context?.previousWhitelist) {
        queryClient.setQueryData(
          ['whitelist', serverId],
          context.previousWhitelist
        );
      }
      toast.error(error.message);
    },
  });
}
