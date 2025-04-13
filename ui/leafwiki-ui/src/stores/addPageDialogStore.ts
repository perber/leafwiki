// stores/AddPageDialogStore.ts

import { createPage, suggestSlug } from '@/lib/api';
import { handleFieldErrors } from '@/lib/handleFieldErrors';
import { toast } from 'sonner';
import { create } from 'zustand';
import { useTreeStore } from './tree';

type AddPageDialogStore = {
    open: boolean
    parentId: string
    title: string
    slug: string
    loading: boolean
    errors: Record<string, string> | null;
    getSlug: (parentId: string, title: string) => Promise<void>
    openDialog: (parentId: string) => void
    closeDialog: () => void
    setTitle: (title: string) => void
    setFieldErrors: (errors: Record<string, string>) => void;
    setSlug: (slug: string) => void
    resetState: () => void
    createPage: () => Promise<void>
}

export const useAddPageDialogStore = create<AddPageDialogStore>((set, get) => ({
    open: false,
    parentId: '',
    title: '',
    slug: '',
    loading: false,
    errors: null,
    setFieldErrors: (errors) => set({ errors }),
    openDialog: (parentId) => set({ parentId: parentId, title: "", slug: "", open: true, errors: null }),
    closeDialog: () => set({ parentId: "", title: "", slug: "", open: false, errors: null }),
    setTitle: (title) => set({ title, errors: null }),
    setSlug: (slug) => set({ slug, errors: null }),
    getSlug: async (parentId, title) => {
        try {
            const slug = await suggestSlug(parentId, title)
            set({ slug, errors: null })
        } catch (error) {
            console.warn('Error generating slug:', error)
            set({ slug: '', errors: { slug: "couldn't generate slug" } })
            toast.error('Error generating slug')
        }
    },
    resetState: () => set({ title: '', slug: '', parentId: "", open: false, errors: null, loading: false, }),
    createPage: async () => {
        try {
            set({ loading: true, errors: null })
            await createPage({ title: get().title, slug: get().slug, parentId: get().parentId })
            set({ loading: false })
            toast.success('Page created')
            // Reload the tree or navigate to the new page
            useTreeStore.getState().reloadTree()
            set({ open: false, title: '', slug: '', parentId: "", errors: null })
        } catch (error) {
            console.warn('Error creating page:', error)
            handleFieldErrors(error, get().setFieldErrors, 'Error creating page');
        } finally {
            set({ loading: false })
        }

    },
}))
