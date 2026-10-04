// One form's builder. Keyed on the form id so switching forms remounts the
// editor instead of writing one form's draft into another (same pattern as
// the automation editor).

import { useParams } from "@tanstack/react-router";

import { EmptyBlock } from "@/components/layout/Page";
import { NoAccess } from "@/components/layout/NoAccess";
import FormBuilder from "@/components/app/forms/FormBuilder";
import { usePermission } from "@/hooks/usePermission";
import { useForm } from "@/lib/api/hooks/app/forms";

export default function FormBuilderPage() {
    const canView = usePermission("VIEW_CONTACTS");
    const { id } = useParams({ from: "/app/forms/$id" });
    const form = useForm(canView ? id : undefined);

    if (!canView) return <NoAccess feature="forms" permissionLabel="View contacts" />;

    if (form.isPending) {
        return (
            <div className="p-6 space-y-3">
                <div className="h-8 w-64 rounded-md bg-slate-100 animate-pulse" />
                <div className="h-96 rounded-md bg-slate-100 animate-pulse" />
            </div>
        );
    }
    if (form.isError || !form.data) {
        return <EmptyBlock title="Form not found" body="It may have been deleted by a teammate." />;
    }
    return <FormBuilder key={form.data.id} form={form.data} />;
}
