export type VirtualizedRange = {
    startIndex: number;
    endIndex: number;
    offsetTop: number;
    totalHeight: number;
};

type VirtualizedRangeOptions = {
    itemCount: number;
    scrollTop: number;
    viewportHeight: number;
    rowHeight: number;
    overscan: number;
    stickyOffset?: number;
};

export const DEFAULT_TABLE_OVERSCAN = 10;
const FALLBACK_VISIBLE_ROW_COUNT = 24;

export function calculateVirtualizedRange(
    options: VirtualizedRangeOptions,
): VirtualizedRange {
    const {
        itemCount,
        scrollTop,
        viewportHeight,
        rowHeight,
        overscan,
        stickyOffset = 0,
    } = options;

    const totalHeight = itemCount * rowHeight;
    if (itemCount === 0 || rowHeight <= 0) {
        return {
            startIndex: 0,
            endIndex: itemCount,
            offsetTop: 0,
            totalHeight,
        };
    }

    const effectiveViewportHeight =
        viewportHeight > 0
            ? viewportHeight
            : rowHeight * FALLBACK_VISIBLE_ROW_COUNT + stickyOffset;
    const bodyViewportHeight = Math.max(0, effectiveViewportHeight - stickyOffset);
    const bodyScrollTop = Math.max(0, scrollTop - stickyOffset);
    const startIndex = Math.max(
        0,
        Math.floor(bodyScrollTop / rowHeight) - overscan,
    );
    const visibleRowCount =
        Math.ceil(bodyViewportHeight / rowHeight) + overscan * 2;
    const endIndex = Math.min(itemCount, startIndex + visibleRowCount);

    return {
        startIndex,
        endIndex,
        offsetTop: startIndex * rowHeight,
        totalHeight,
    };
}
