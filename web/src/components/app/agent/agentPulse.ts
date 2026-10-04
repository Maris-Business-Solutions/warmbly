// A keystroke signal from the assistant composer to any AgentMark rendered with
// `listen`, so the mark bounces while someone types to it.

const subscribers = new Set<() => void>();

export function pulseAgent() {
    subscribers.forEach((fn) => fn());
}

export function subscribeAgentPulse(fn: () => void): () => void {
    subscribers.add(fn);
    return () => {
        subscribers.delete(fn);
    };
}
