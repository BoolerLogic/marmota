import type {
    RepeaterHeaderMap,
    RepeaterHttp2PseudoHeaders,
    RepeaterHttpVersion,
    RepeaterScheme,
} from "./repeaterRequest";

export type RepeaterSendPayload = {
    scheme: RepeaterScheme;
    host: string;
    port: string;
    method: string;
    path: string;
    headBlockStr: string;
    bodyStr: string;
    skipServerCertVerify: boolean;
    version: RepeaterHttpVersion;
    pseudoHeaders: RepeaterHttp2PseudoHeaders;
    headers: RepeaterHeaderMap;
};

export type RepeaterSendResult = {
    headBlockStr: string;
    bodyStr: string;
    host?: string;
    port?: string;
    version?: string;
    statusCode?: number | null;
    durationMs?: number | null;
};

type RepeaterWindow = Window & {
    go?: {
        main?: {
            App?: {
                SendRepeaterRequest?: (
                    payload: RepeaterSendPayload,
                ) => Promise<RepeaterSendResult>;
            };
        };
    };
};

export async function sendRepeaterRequest(
    payload: RepeaterSendPayload,
): Promise<RepeaterSendResult> {
    const repeaterWindow = window as RepeaterWindow;
    const sendFn = repeaterWindow.go?.main?.App?.SendRepeaterRequest;

    if (typeof sendFn !== "function") {
        throw new Error(
            "Repeater is not connected to the backend yet.",
        );
    }

    return sendFn(payload);
}
