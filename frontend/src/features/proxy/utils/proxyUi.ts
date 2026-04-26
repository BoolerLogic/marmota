export type ProxyState = "idle" | "loading" | "running" | "error";
export type ProxyMode = "local" | "all" | "specific";

export const PORT_MIN = 0;
export const PORT_MAX = 65535; // Standard TCP/UDP port range.

export function resolveListenIp(ipMode: ProxyMode, specificIp: string): string {
  if (ipMode === "local") return "127.0.0.1";
  if (ipMode === "all") return "0.0.0.0";
  return specificIp.trim();
}

export function normalizePortText(portText: string | number | null | undefined): string {
  return String(portText ?? "").trim();
}

export function parsePort(portText: string | number | null | undefined): number | null {
  const s = normalizePortText(portText);
  if (s.length === 0) return null;

  const n = Number(s);
  if (!Number.isFinite(n)) return null;
  if (!Number.isInteger(n)) return null;

  return n;
}

export function isValidIP(str: string): boolean {
  const regex = /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]\d?|0)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]\d?|0)){3}$/;
  return regex.test(str);
};

export function isValidPort(port: number | null): boolean {
  if (port === null) return false;
  return port >= PORT_MIN && port <= PORT_MAX;
}
