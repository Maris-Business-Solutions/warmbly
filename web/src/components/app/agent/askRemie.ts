// askRemie opens Remie on a fresh conversation and sends `prompt` there, for
// "Ask Remie" buttons outside the panel. The panel picks the question up once
// its tab is ready (see agentQueuedPrompt), so this never races hydration.

import { useAppStore } from "@/stores";

export function askRemie(prompt: string) {
    const s = useAppStore.getState();
    const active = s.agentTabs.find((t) => t.key === s.agentActiveKey);
    // Reuse an untouched tab rather than stacking empty ones; a typed draft counts as touched.
    const blank =
        active && !active.sessionId && active.turns.length === 0 && !active.running && !active.draft.trim();
    if (!blank) s.agentNewTab();
    s.agentQueuePrompt(prompt);
    s.setAgentMinimized(false);
    s.setAIAssistantOpen(true);
}
