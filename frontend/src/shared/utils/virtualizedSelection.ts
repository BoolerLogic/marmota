export type SelectionMoveDirection = -1 | 1;

type AdjacentVirtualizedSelectionOptions<T> = {
    items: T[];
    selectedId: string | null | undefined;
    getId: (item: T) => string;
    direction: SelectionMoveDirection;
};

export type AdjacentVirtualizedSelection = {
    id: string;
    index: number;
};

type ScrollVirtualizedRowIntoViewOptions = {
    viewport: HTMLElement | null;
    rowIndex: number;
    rowHeight: number;
    stickyOffset?: number;
};

export function getAdjacentVirtualizedSelection<T>(
    options: AdjacentVirtualizedSelectionOptions<T>,
): AdjacentVirtualizedSelection | null {
    const { items, selectedId, getId, direction } = options;
    if (items.length === 0) {
        return null;
    }

    const currentIndex =
        selectedId == null
            ? -1
            : items.findIndex((item) => getId(item) === selectedId);
    const fallbackIndex = direction > 0 ? 0 : items.length - 1;
    const unclampedNextIndex =
        currentIndex === -1 ? fallbackIndex : currentIndex + direction;
    const nextIndex = Math.max(0, Math.min(unclampedNextIndex, items.length - 1));

    if (nextIndex === currentIndex) {
        return null;
    }

    return {
        id: getId(items[nextIndex]),
        index: nextIndex,
    };
}

export function scrollVirtualizedRowIntoView(
    options: ScrollVirtualizedRowIntoViewOptions,
) {
    const { viewport, rowIndex, rowHeight, stickyOffset = 0 } = options;
    if (!viewport || rowIndex < 0 || rowHeight <= 0) {
        return;
    }

    const rowTop = stickyOffset + rowIndex * rowHeight;
    const rowBottom = rowTop + rowHeight;
    const visibleTop = viewport.scrollTop + stickyOffset;
    const visibleBottom = viewport.scrollTop + viewport.clientHeight;

    if (rowTop < visibleTop) {
        viewport.scrollTop = Math.max(0, rowTop - stickyOffset);
        return;
    }

    if (rowBottom > visibleBottom) {
        viewport.scrollTop = Math.max(0, rowBottom - viewport.clientHeight);
    }
}
