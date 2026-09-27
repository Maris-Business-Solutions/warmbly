// Browser page translation swaps React's text nodes, so skip mismatched removes/inserts instead of crashing (facebook/react#11538).
export function installDomMutationGuard(): void {
    if (typeof Node !== "function" || !Node.prototype) return;

    const originalRemoveChild = Node.prototype.removeChild;
    Node.prototype.removeChild = function <T extends Node>(this: Node, child: T): T {
        if (child.parentNode !== this) {
            if (import.meta.env.DEV) console.warn("removeChild skipped: node belongs to another parent", child, this);
            return child;
        }
        return originalRemoveChild.call(this, child) as T;
    };

    const originalInsertBefore = Node.prototype.insertBefore;
    Node.prototype.insertBefore = function <T extends Node>(this: Node, node: T, child: Node | null): T {
        if (child && child.parentNode !== this) {
            if (import.meta.env.DEV) console.warn("insertBefore skipped: reference node belongs to another parent", child, this);
            return node;
        }
        return originalInsertBefore.call(this, node, child) as T;
    };
}
