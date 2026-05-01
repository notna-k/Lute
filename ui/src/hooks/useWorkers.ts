import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  workerService,
  CreateWorkerRequest,
  UpdateWorkerRequest,
} from '../services/workerService';

export const workerKeys = {
  all: ['workers'] as const,
  lists: () => [...workerKeys.all, 'list'] as const,
  list: (filter: string) => [...workerKeys.lists(), filter] as const,
  details: () => [...workerKeys.all, 'detail'] as const,
  detail: (id: string) => [...workerKeys.details(), id] as const,
};

export const useUserWorkers = () => {
  return useQuery({
    queryKey: workerKeys.list('user'),
    queryFn: workerService.getUserWorkers,
    staleTime: 30000,
    gcTime: 5 * 60 * 1000,
  });
};

export const useWorker = (id: string) => {
  return useQuery({
    queryKey: workerKeys.detail(id),
    queryFn: () => workerService.getWorker(id),
    enabled: !!id,
    staleTime: 30000,
  });
};

export const useCreateWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWorkerRequest) => workerService.createWorker(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: workerKeys.list('user') });
    },
  });
};

export const useUpdateWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWorkerRequest }) =>
      workerService.updateWorker(id, data),
    onSuccess: (data) => {
      queryClient.setQueryData(workerKeys.detail(data.id), data);
      queryClient.invalidateQueries({ queryKey: workerKeys.lists() });
    },
  });
};

export const useReEnableWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => workerService.reEnableWorker(id),
    onSuccess: (data) => {
      queryClient.setQueryData(workerKeys.detail(data.id), data);
      queryClient.invalidateQueries({ queryKey: workerKeys.lists() });
    },
  });
};

export const useDeleteWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => workerService.deleteWorker(id),
    onSuccess: (_, deletedId) => {
      queryClient.removeQueries({ queryKey: workerKeys.detail(deletedId) });
      queryClient.invalidateQueries({ queryKey: workerKeys.lists() });
    },
  });
};
