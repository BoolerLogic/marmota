import { GetHistoryEntryDetail } from "../../../../wailsjs/go/main/App.js";
import type { bridge } from "../../../../wailsjs/go/models";

export type RequestView = {
    id: string;
    host: string;
    port: string;
    headBlockStr: string;
    bodyStr: string;
    truncatedBody?: boolean;
    version: string;
    method: string;
    path: string;
    scheme: string;
};

export type ResponseView = {
    id: string;
    host: string;
    port: string;
    headBlockStr: string;
    bodyStr: string;
    truncatedBody?: boolean;
    version: string;
    statusCode: number | null;
    unsupportedContentEncodings: string[];
    contentDecodingFailed: boolean;
};

export type HistoryEntryDetail = {
    id: string;
    request: RequestView | null;
    response: ResponseView | null;
};

export const HISTORY_DETAIL_CACHE_LIMIT = 30;

const detailCache = new Map<string, HistoryEntryDetail>();
const detailRequestsInFlight = new Map<string, Promise<HistoryEntryDetail>>();
type PendingDetailRequestState = {
    invalidated: boolean;
    refreshAfterInvalidation: boolean;
};
const pendingDetailRequestStatesById = new Map<
    string,
    Set<PendingDetailRequestState>
>();

function registerPendingDetailRequest(
    id: string,
    requestState: PendingDetailRequestState,
) {
    const pendingRequestStates =
        pendingDetailRequestStatesById.get(id) ??
        new Set<PendingDetailRequestState>();
    pendingRequestStates.add(requestState);
    pendingDetailRequestStatesById.set(id, pendingRequestStates);
}

function unregisterPendingDetailRequest(
    id: string,
    requestState: PendingDetailRequestState,
) {
    const pendingRequestStates = pendingDetailRequestStatesById.get(id);
    if (!pendingRequestStates) {
        return;
    }

    pendingRequestStates.delete(requestState);
    if (pendingRequestStates.size === 0) {
        pendingDetailRequestStatesById.delete(id);
    }
}

function invalidatePendingDetailRequests(
    id: string,
    refreshAfterInvalidation: boolean,
) {
    const pendingRequestStates = pendingDetailRequestStatesById.get(id);
    if (!pendingRequestStates) {
        return;
    }

    for (const requestState of pendingRequestStates) {
        requestState.invalidated = true;
        requestState.refreshAfterInvalidation = refreshAfterInvalidation;
    }
}

function normalizeRequestDetail(
    payload: bridge.HTTPRequestDetail | null | undefined,
): RequestView | null {
    if (!payload) {
        return null;
    }

    return {
        id: String(payload.id),
        host: payload.host ?? "",
        port: payload.port ?? "",
        headBlockStr: payload.headBlockStr ?? "",
        bodyStr: payload.bodyStr ?? "",
        truncatedBody: payload.truncatedBody ?? false,
        version: payload.version ?? "",
        method: payload.method ?? "",
        path: payload.path ?? "",
        scheme: payload.scheme ?? "",
    };
}

function normalizeResponseDetail(
    payload: bridge.HTTPResponseDetail | null | undefined,
): ResponseView | null {
    if (!payload) {
        return null;
    }

    return {
        id: String(payload.id),
        host: payload.host ?? "",
        port: payload.port ?? "",
        headBlockStr: payload.headBlockStr ?? "",
        bodyStr: payload.bodyStr ?? "",
        truncatedBody: payload.truncatedBody ?? false,
        version: payload.version ?? "",
        statusCode: payload.statusCode ?? null,
        unsupportedContentEncodings: Array.isArray(
            payload.unsupportedContentEncodings,
        )
            ? payload.unsupportedContentEncodings
                  .filter(
                      (value): value is string =>
                          typeof value === "string",
                  )
                  .map((value) => value.trim().toLowerCase())
                  .filter(Boolean)
            : [],
        contentDecodingFailed: payload.contentDecodingFailed ?? false,
    };
}

function normalizeHistoryEntryDetail(
    payload: bridge.HTTPHistoryEntryDetail,
): HistoryEntryDetail {
    return {
        id: String(payload.id),
        request: normalizeRequestDetail(payload.request),
        response: normalizeResponseDetail(payload.response),
    };
}

function setCachedHistoryEntryDetail(detail: HistoryEntryDetail) {
    // An entry without either side is the backend's safe "not found" response.
    // It may be observed by a request that raced with deletion, but it must not
    // become a durable cache hit.
    if (!detail.request && !detail.response) {
        detailCache.delete(detail.id);
        return;
    }

    detailCache.delete(detail.id);
    detailCache.set(detail.id, detail);

    while (detailCache.size > HISTORY_DETAIL_CACHE_LIMIT) {
        const oldestCacheKey = detailCache.keys().next().value;
        if (!oldestCacheKey) {
            break;
        }

        detailCache.delete(oldestCacheKey);
    }
}

export function getCachedHistoryEntryDetail(
    id: string,
): HistoryEntryDetail | null {
    const cachedDetail = detailCache.get(id);
    if (!cachedDetail) {
        return null;
    }

    setCachedHistoryEntryDetail(cachedDetail);
    return cachedDetail;
}

export async function getHistoryEntryDetailCached(
    id: string,
): Promise<HistoryEntryDetail> {
    const cachedDetail = getCachedHistoryEntryDetail(id);
    if (cachedDetail) {
        return cachedDetail;
    }

    const currentRequest = detailRequestsInFlight.get(id);
    if (currentRequest) {
        return currentRequest;
    }

    const requestState: PendingDetailRequestState = {
        invalidated: false,
        refreshAfterInvalidation: false,
    };
    const nextRequest: Promise<HistoryEntryDetail> = GetHistoryEntryDetail(
        toBackendHistoryId(id),
    )
        .then(normalizeHistoryEntryDetail)
        .then((detail) => {
            if (requestState.invalidated) {
                return requestState.refreshAfterInvalidation
                    ? getHistoryEntryDetailCached(id)
                    : detail;
            }
            if (detailRequestsInFlight.get(id) !== nextRequest) {
                return detail;
            }

            setCachedHistoryEntryDetail(detail);
            return detail;
        })
        .finally(() => {
            if (detailRequestsInFlight.get(id) === nextRequest) {
                detailRequestsInFlight.delete(id);
            }
            unregisterPendingDetailRequest(id, requestState);
        });

    registerPendingDetailRequest(id, requestState);
    detailRequestsInFlight.set(id, nextRequest);
    return nextRequest;
}

export function cacheHistoryEntryDetail(detail: HistoryEntryDetail) {
    setCachedHistoryEntryDetail(detail);
}

export function invalidateHistoryEntryDetail(id: string) {
    invalidatePendingDetailRequests(id, true);
    detailCache.delete(id);
    detailRequestsInFlight.delete(id);
}

export function removeHistoryEntryDetail(id: string) {
    // Unlike an update invalidation, deletion must not make an older request
    // issue a follow-up fetch for an entry that no longer exists.
    invalidatePendingDetailRequests(id, false);
    detailCache.delete(id);
    detailRequestsInFlight.delete(id);
}

export function clearHistoryDetailCache() {
    for (const pendingRequestStates of pendingDetailRequestStatesById.values()) {
        for (const requestState of pendingRequestStates) {
            requestState.invalidated = true;
            requestState.refreshAfterInvalidation = false;
        }
    }

    detailCache.clear();
    detailRequestsInFlight.clear();
    pendingDetailRequestStatesById.clear();
}

function toBackendHistoryId(id: string): number {
    const parsedId = Number(id);
    if (!Number.isFinite(parsedId)) {
        throw new Error(`Could not convert ID "${id}" for the backend.`);
    }

    return parsedId;
}
