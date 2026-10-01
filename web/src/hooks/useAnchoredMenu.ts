// A PopoverMenu that opens under its own trigger on a click and at the pointer
// on a right-click. Spread `menuProps` onto the PopoverMenu and put
// `onContextMenu` on the element the right-click belongs to.

import React from "react";

export type AnchorPoint = { x: number; y: number };

export function useAnchoredMenu() {
  const [open, setOpen] = React.useState(false);
  const [point, setPoint] = React.useState<AnchorPoint | null>(null);
  const onContextMenu = (e: React.MouseEvent<HTMLElement>) => {
    e.preventDefault();
    e.stopPropagation();
    // A keyboard context-menu key reports 0,0: open under the element instead.
    const r = e.currentTarget.getBoundingClientRect();
    setPoint(e.clientX || e.clientY ? { x: e.clientX, y: e.clientY } : { x: r.left + 12, y: r.bottom });
    setOpen(true);
  };
  const menuProps = {
    open,
    anchorPoint: point,
    onOpenChange: (o: boolean) => {
      if (o) setPoint(null);
      setOpen(o);
    },
  };
  // `fromPointer` tells a right-click from the trigger, for menus that act on
  // more than their own row when right-clicked.
  return { open, fromPointer: open && point !== null, onContextMenu, menuProps };
}
