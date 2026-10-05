// Catch-all for paths that are not part of the admin app.

import { Link } from "react-router-dom";
import { ArrowLeft, FileQuestion } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/kit";

export default function NotFoundPage() {
    return (
        <div>
            <PageHeader title="Page not found" icon={FileQuestion} />
            <EmptyState
                icon={FileQuestion}
                title="Page not found"
                hint="That route is not part of the admin app. It may have moved; the sidebar and the search palette list every page."
                action={
                    <Button size="sm" variant="outline" asChild>
                        <Link to="/">
                            <ArrowLeft />
                            Back to overview
                        </Link>
                    </Button>
                }
                className="py-24"
            />
        </div>
    );
}
