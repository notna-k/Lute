import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { machineService, CreateMachineRequest, UpdateMachineRequest } from '../services/machineService';

// Query keys
export const machineKeys = {
    all: ['machines'] as const,
    lists: () => [...machineKeys.all, 'list'] as const,
    list: (filter: string) => [...machineKeys.lists(), filter] as const,
    details: () => [...machineKeys.all, 'detail'] as const,
    detail: (id: string) => [...machineKeys.details(), id] as const,
};

// Get user's machines
export const useUserMachines = () => {
    return useQuery({
        queryKey: machineKeys.list('user'),
        queryFn: machineService.getUserMachines,
        staleTime: 30000, // Consider data fresh for 30 seconds
        gcTime: 5 * 60 * 1000, // Keep in cache for 5 minutes
    });
};

// Get public machines
export const usePublicMachines = () => {
    return useQuery({
        queryKey: machineKeys.list('public'),
        queryFn: machineService.getPublicMachines,
        staleTime: 60000, // Consider data fresh for 1 minute (public data changes less frequently)
        gcTime: 10 * 60 * 1000, // Keep in cache for 10 minutes
    });
};

// Get single machine
export const useMachine = (id: string) => {
    return useQuery({
        queryKey: machineKeys.detail(id),
        queryFn: () => machineService.getMachine(id),
        enabled: !!id, // Only fetch if ID is provided
        staleTime: 30000,
    });
};

// Create machine mutation
export const useCreateMachine = () => {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (data: CreateMachineRequest) => machineService.createMachine(data),
        onSuccess: () => {
            // Invalidate and refetch user machines list
            queryClient.invalidateQueries({ queryKey: machineKeys.list('user') });
        },
    });
};

// Update machine mutation
export const useUpdateMachine = () => {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: ({ id, data }: { id: string; data: UpdateMachineRequest }) =>
            machineService.updateMachine(id, data),
        onSuccess: (data) => {
            // Update the specific machine in cache
            queryClient.setQueryData(machineKeys.detail(data.id), data);
            // Invalidate lists to refetch
            queryClient.invalidateQueries({ queryKey: machineKeys.lists() });
        },
    });
};

// Re-enable a dead machine (sets status to "pending" so the agent can connect again)
export const useReEnableMachine = () => {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (id: string) => machineService.reEnableMachine(id),
        onSuccess: (data) => {
            queryClient.setQueryData(machineKeys.detail(data.id), data);
            queryClient.invalidateQueries({ queryKey: machineKeys.lists() });
        },
    });
};

// Delete machine mutation
export const useDeleteMachine = () => {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (id: string) => machineService.deleteMachine(id),
        onSuccess: (_, deletedId) => {
            // Remove from cache
            queryClient.removeQueries({ queryKey: machineKeys.detail(deletedId) });
            // Invalidate lists to refetch
            queryClient.invalidateQueries({ queryKey: machineKeys.lists() });
        },
    });
};

