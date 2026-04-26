const scrollMemory = new Map<string, number>();

export function rememberScrollPosition(
    key: string,
    scrollTop: number,
) {
    scrollMemory.set(key, Math.max(0, scrollTop));
}

export function getRememberedScrollPosition(key: string): number {
    return scrollMemory.get(key) ?? 0;
}

export function forgetScrollPosition(key: string) {
    scrollMemory.delete(key);
}
