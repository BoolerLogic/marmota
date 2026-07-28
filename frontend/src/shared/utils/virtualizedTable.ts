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

function normalizeNonNegativeInteger(value: number): number {
    if (!Number.isFinite(value)) {
        return 0;
    }

    return Math.max(0, Math.floor(value));
}

function normalizeNonNegativeFiniteNumber(value: number): number {
    if (!Number.isFinite(value) || value <= 0) {
        return 0;
    }

    return value;
}

export function clampVirtualizedScrollTop(
    scrollTop: number,
    scrollHeight: number,
    viewportHeight: number,
): number {
    const safeScrollHeight =
        normalizeNonNegativeFiniteNumber(scrollHeight);
    const safeViewportHeight =
        normalizeNonNegativeFiniteNumber(viewportHeight);
    const maximumScrollTop = Math.max(
        0,
        safeScrollHeight - safeViewportHeight,
    );
    const safeScrollTop =
        scrollTop === Number.POSITIVE_INFINITY
            ? maximumScrollTop
            : normalizeNonNegativeFiniteNumber(scrollTop);

    return Math.min(safeScrollTop, maximumScrollTop);
}

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

    const safeItemCount = normalizeNonNegativeInteger(itemCount);
    const safeRowHeight =
        Number.isFinite(rowHeight) && rowHeight > 0 ? rowHeight : 0;
    const totalHeight = safeItemCount * safeRowHeight;
    if (safeItemCount === 0 || safeRowHeight === 0) {
        return {
            startIndex: 0,
            endIndex: safeItemCount,
            offsetTop: 0,
            totalHeight,
        };
    }

    const safeStickyOffset =
        normalizeNonNegativeFiniteNumber(stickyOffset);
    const safeOverscan = normalizeNonNegativeInteger(overscan);
    const effectiveViewportHeight =
        Number.isFinite(viewportHeight) && viewportHeight > 0
            ? viewportHeight
            : safeRowHeight * FALLBACK_VISIBLE_ROW_COUNT + safeStickyOffset;
    const bodyViewportHeight = Math.max(
        0,
        effectiveViewportHeight - safeStickyOffset,
    );
    const maximumScrollTop = Math.max(
        0,
        safeStickyOffset + totalHeight - effectiveViewportHeight,
    );
    const clampedScrollTop = Math.min(
        scrollTop === Number.POSITIVE_INFINITY
            ? maximumScrollTop
            : normalizeNonNegativeFiniteNumber(scrollTop),
        maximumScrollTop,
    );
    // A sticky header still occupies normal-flow space before the body. At
    // scrollTop N, the first body pixel visible beneath it is also N.
    const bodyScrollTop = clampedScrollTop;
    const firstVisibleIndex = Math.floor(
        bodyScrollTop / safeRowHeight,
    );
    const visibleEndIndex = Math.ceil(
        (bodyScrollTop + bodyViewportHeight) / safeRowHeight,
    );
    const unclampedStartIndex = firstVisibleIndex - safeOverscan;
    const startIndex = Math.min(
        safeItemCount - 1,
        Math.max(0, unclampedStartIndex),
    );
    const endIndex = Math.min(
        safeItemCount,
        Math.max(startIndex + 1, visibleEndIndex + safeOverscan),
    );

    return {
        startIndex,
        endIndex,
        offsetTop: startIndex * safeRowHeight,
        totalHeight,
    };
}
