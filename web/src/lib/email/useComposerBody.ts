import React from "react";
import { htmlToPlain, restoreBody, withHtml, type ComposerBody } from "./composerBody";

/** State for a composer body that can switch between plain text and HTML. */
export function useComposerBody(plain: string, html?: string | null) {
    const [value, setValue] = React.useState<ComposerBody>(() => restoreBody(plain, html));
    const setBody = React.useCallback(
        (next: string | ((prev: string) => string)) =>
            setValue((v) => ({ ...v, plain: typeof next === "function" ? next(v.plain) : next })),
        [],
    );
    const setHtml = React.useCallback((next: string) => setValue((v) => withHtml(v, next)), []);
    const setSync = React.useCallback(
        (on: boolean) =>
            setValue((v) => ({ ...v, sync: on, plain: on && v.html !== null ? htmlToPlain(v.html) : v.plain })),
        [],
    );
    return { value, body: value.plain, html: value.html, sync: value.sync, setValue, setBody, setHtml, setSync };
}

export type ComposerBodyState = ReturnType<typeof useComposerBody>;
