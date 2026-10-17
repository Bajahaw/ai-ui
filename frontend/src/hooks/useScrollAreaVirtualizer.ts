import { useLayoutEffect, useRef, useState, type RefObject } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

const SCROLL_AREA_VIEWPORT = "[data-radix-scroll-area-viewport]";

const layoutTop = (el: HTMLElement) => {
  let top = 0;
  for (let node: HTMLElement | null = el; node; node = node.offsetParent as HTMLElement | null) {
    top += node.offsetTop;
  }
  return top;
};

export function useScrollAreaVirtualizer({
  listRef,
  count,
  estimateSize,
  overscan,
}: {
  listRef: RefObject<HTMLElement | null>;
  count: number;
  estimateSize: (index: number) => number;
  overscan?: number;
}) {
  const [scrollMargin, setScrollMargin] = useState(0);
  const viewportRef = useRef<HTMLElement | null>(null);
  const hasList = count > 0;
  const findViewport = () => {
    const viewport = listRef.current?.closest<HTMLElement>(SCROLL_AREA_VIEWPORT);
    if (viewport) viewportRef.current = viewport;
    return viewportRef.current;
  };

  useLayoutEffect(() => {
    const list = listRef.current;
    const content = list?.closest(SCROLL_AREA_VIEWPORT)?.firstElementChild;
    if (!hasList || !list || !(content instanceof HTMLElement)) return;
    viewportRef.current = content.parentElement;
    const measure = () => setScrollMargin(layoutTop(list) - layoutTop(content));
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    return () => observer.disconnect();
  }, [hasList, listRef]);

  return useVirtualizer({
    count,
    getScrollElement: findViewport,
    estimateSize,
    overscan,
    scrollMargin,
  });
}
