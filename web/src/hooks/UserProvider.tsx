import { clearClientSession } from "@/lib/session";
import { SESSION_ENDED_EVENT } from "@/lib/auth";
import { TOKEN_KEY } from "@/lib/information";
import getToken from "@/lib/helper/getToken";
import React, { useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { UserContext } from './context/user';
import { useUserProfile } from './context/user';
import useAuthUser from '@/lib/api/hooks/auth/useUser';
import { AnimatePresence } from 'framer-motion';
import LoadingScreen from '@/components/LoadingScreen';
import useRoles from '@/lib/api/hooks/app/admin/roles/useRoles';
import useTimezones from '@/lib/api/hooks/app/useTimezones';
import type { AppError } from '@/lib/api/client/normalizeError';
import { AuthError } from '@/lib/errors/auth';
import { Navigate } from '@tanstack/react-router';
import type Access from '@/lib/api/models/app/admin/Access';
import type Timezone from '@/lib/api/models/app/Timezone';
import type User from '@/lib/api/models/auth/User';

const EMPTY_ACCESS: Access = { roles: [], permissions: [] };
const EMPTY_TIMEZONES: Timezone[] = [];

export const UserProvider = ({ children }: { children: React.ReactNode }) => {
    const queryClient = useQueryClient();
    const user = useAuthUser();
    const access = useRoles();
    const timezones = useTimezones();

    const [tagsEdit, setTagsEdit] = React.useState<boolean>(false);
    const [foldersEdit, setFoldersEdit] = React.useState<boolean>(false);
    const [addEmail, setAddEmail] = React.useState<boolean>(false);
    const safeUser = useMemo((): User | null => {
        if (!user.data) return null;
        return {
            ...user.data,
            tags: Array.isArray(user.data.tags) ? user.data.tags : [],
            categories: Array.isArray(user.data.categories) ? user.data.categories : [],
            folders: Array.isArray(user.data.folders) ? user.data.folders : [],
            roles: Array.isArray(user.data.roles) ? user.data.roles : [],
        };
    }, [user.data]);

    // The api client ends a session from outside React, and the three queries
    // below are cached and never poll, so without this the tokens went and the
    // app stayed put on a page it could no longer authenticate.
    const [sessionEnded, setSessionEnded] = React.useState(false);
    React.useEffect(() => {
        const ended = () => setSessionEnded(true);
        window.addEventListener(SESSION_ENDED_EVENT, ended);
        // Signing out in one tab takes the token away from all of them, and the
        // event above is dispatched only in the tab that did it. The storage
        // event fires only in the others, which is exactly the set left holding
        // a rendered app it can no longer authenticate. A null key is storage
        // being cleared wholesale.
        const stored = (e: StorageEvent) => {
            if (e.key !== null && e.key !== TOKEN_KEY) return;
            if (!getToken()) setSessionEnded(true);
        };
        window.addEventListener("storage", stored);
        return () => {
            window.removeEventListener(SESSION_ENDED_EVENT, ended);
            window.removeEventListener("storage", stored);
        };
    }, []);

    const error = useMemo(() => {
        const errs = [user.error, access.error, timezones.error].filter(Boolean);
        for (const err of errs) {
            if (err instanceof AuthError) {
                return { redirect: true, title: "Authentication Required", message: err.message };
            }
            const myerr = err as unknown as AppError;
            if (myerr.status === 401 || myerr.redirect) {
                return { redirect: true, title: "Authentication Required", message: myerr.message ?? "Session expired." };
            }
        }

        if (user.error) {
            const myerr = user.error as unknown as AppError;
            return {
                title: `${myerr.error ?? "Error"}${myerr.status ? ` (${myerr.status})` : ""}`,
                message: myerr.message ?? "An unexpected error occurred.",
            };
        }
    }, [user.error, access.error, timezones.error]);

    const setUser = React.useCallback<React.Dispatch<React.SetStateAction<User | null>>>((value) => {
        queryClient.setQueryData<User | null>(["auth", "me"], (old) => (
            typeof value === "function"
                ? (value as (prev: User | null) => User | null)(old ?? null)
                : value
        ));
    }, [queryClient]);

    if (sessionEnded || error?.redirect) {
        // Same teardown as an explicit sign-out: being signed out must not
        // leave the previous person's drafts and workspace selection behind.
        clearClientSession(queryClient);
        return <Navigate to="/auth/login" replace />;
    }

    if (safeUser && !safeUser.onboarding_completed_at) {
        return <Navigate to="/onboarding" replace />;
    }

    if (!safeUser) {
        return (
            <AnimatePresence>
                <LoadingScreen
                    errorTitle={error?.title}
                    errorMessage={error?.message}
                />
            </AnimatePresence>
        );
    }

    // NOTE: TagsModal / FoldersModal / AddEmailModal are NOT rendered
    // here anymore. They depend on ConfirmContext (via useConfirm),
    // which lives below this provider in the tree. Mounting them here
    // put them outside the ConfirmProvider and crashed with
    // "ConfirmProvider not found" on first interaction. They're now
    // rendered in app/app/layout.tsx after ConfirmProvider is in scope.
    return (
        <UserContext.Provider value={{
            user: safeUser,
            access: access.data ?? EMPTY_ACCESS,
            timezones: timezones.data ?? EMPTY_TIMEZONES,
            setUser,
            tagsEdit,
            setTagsEdit,
            foldersEdit,
            setFoldersEdit,
            addEmail,
            setAddEmail,
        }}>
            {children}
        </UserContext.Provider>
    );
};

export { useUserProfile as useUser };
export type { default as Folder } from "@/lib/api/models/app/Folder";
export type { default as Tag } from "@/lib/api/models/app/Tag";
export type { default as User } from "@/lib/api/models/auth/User";
