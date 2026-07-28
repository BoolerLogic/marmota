export type ProxyState = "idle" | "loading" | "running" | "error";
export type ProxyMode = "local" | "all" | "specific";
export type UpstreamProxySettings = {
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password: string;
};

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

export function isValidUpstreamProxyHost(host: string): boolean {
  const normalizedHost = host.trim();
  const encodedLength = new TextEncoder().encode(normalizedHost).length;
  if (
    normalizedHost.length === 0 ||
    normalizedHost.length > 253 ||
    encodedLength > 255
  ) {
    return false;
  }

  if (/[\/\\\s]/.test(normalizedHost)) return false;

  if (!normalizedHost.includes(":")) return true;

  const hasOpeningBracket = normalizedHost.startsWith("[");
  const hasClosingBracket = normalizedHost.endsWith("]");
  if (hasOpeningBracket !== hasClosingBracket) return false;

  const ipv6Host =
    hasOpeningBracket && hasClosingBracket
      ? normalizedHost.slice(1, -1)
      : normalizedHost;
  try {
    new URL(`http://[${ipv6Host}]/`);
    return true;
  } catch {
    return false;
  }
}

export function isValidUpstreamProxyPort(port: number | null): boolean {
  if (port === null) return false;
  return Number.isInteger(port) && port >= 1 && port <= PORT_MAX;
}

export function hasValidUpstreamProxyCredentials(
  username: string,
  password: string,
): boolean {
  const usernameLength = new TextEncoder().encode(username).length;
  const passwordLength = new TextEncoder().encode(password).length;
  const bothEmpty = username.length === 0 && password.length === 0;
  const bothPresent = username.length > 0 && password.length > 0;

  return (
    (bothEmpty || bothPresent) &&
    usernameLength <= 255 &&
    passwordLength <= 255
  );
}

export function isValidUpstreamProxy(
  settings: UpstreamProxySettings,
): boolean {
  if (!settings.enabled) return true;

  return (
    isValidUpstreamProxyHost(settings.host) &&
    isValidUpstreamProxyPort(settings.port) &&
    hasValidUpstreamProxyCredentials(settings.username, settings.password)
  );
}
