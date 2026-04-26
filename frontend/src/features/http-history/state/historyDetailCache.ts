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
};

export type HistoryEntryDetail = {
    id: string;
    request: RequestView | null;
    response: ResponseView | null;
};

export const HISTORY_DETAIL_CACHE_LIMIT = 30;

const detailCache = new Map<string, HistoryEntryDetail>();
const detailRequestsInFlight = new Map<string, Promise<HistoryEntryDetail>>();

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

    const nextRequest = GetHistoryEntryDetail(toBackendHistoryId(id))
        .then(normalizeHistoryEntryDetail)
        .then((detail) => {
            setCachedHistoryEntryDetail(detail);
            return detail;
        })
        .finally(() => {
            detailRequestsInFlight.delete(id);
        });

    detailRequestsInFlight.set(id, nextRequest);
    return nextRequest;
}

export function cacheHistoryEntryDetail(detail: HistoryEntryDetail) {
    setCachedHistoryEntryDetail(detail);
}

export function invalidateHistoryEntryDetail(id: string) {
    detailCache.delete(id);
    detailRequestsInFlight.delete(id);
}

export function clearHistoryDetailCache() {
    detailCache.clear();
    detailRequestsInFlight.clear();
}

function toBackendHistoryId(id: string): number {
    const parsedId = Number(id);
    if (!Number.isFinite(parsedId)) {
        throw new Error(`Could not convert ID "${id}" for the backend.`);
    }

    return parsedId;
}
