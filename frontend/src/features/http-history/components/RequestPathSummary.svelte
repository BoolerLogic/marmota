<script lang="ts">
    type QueryEntry = {
        id: string;
        key: string;
        value: string;
    };

    export let path = "";

    function safeDecode(value: string): string {
        if (!value) return "";

        try {
            return decodeURIComponent(value.replace(/\+/g, " "));
        } catch {
            return value;
        }
    }

    function parsePathParts(rawPath: string): {
        pathname: string;
        queryEntries: QueryEntry[];
        fragment: string;
    } {
        const normalizedPath = rawPath.trim();
        if (!normalizedPath || normalizedPath === "-") {
            return {
                pathname: "-",
                queryEntries: [],
                fragment: "",
            };
        }

        const hashIndex = normalizedPath.indexOf("#");
        const pathWithoutFragment =
            hashIndex >= 0 ? normalizedPath.slice(0, hashIndex) : normalizedPath;
        const fragment =
            hashIndex >= 0 ? normalizedPath.slice(hashIndex + 1) : "";
        const queryIndex = pathWithoutFragment.indexOf("?");
        const pathname =
            queryIndex >= 0
                ? pathWithoutFragment.slice(0, queryIndex) || "/"
                : pathWithoutFragment || "/";
        const rawQuery =
            queryIndex >= 0 ? pathWithoutFragment.slice(queryIndex + 1) : "";
        const queryEntries = rawQuery
            ? rawQuery.split("&").map((entry, index) => {
                  const separatorIndex = entry.indexOf("=");
                  const rawKey =
                      separatorIndex >= 0
                          ? entry.slice(0, separatorIndex)
                          : entry;
                  const rawValue =
                      separatorIndex >= 0 ? entry.slice(separatorIndex + 1) : "";

                  return {
                      id: `query-${index}-${rawKey}`,
                      key: safeDecode(rawKey),
                      value: safeDecode(rawValue),
                  };
              })
            : [];

        return {
            pathname,
            queryEntries,
            fragment: safeDecode(fragment),
        };
    }

    $: parsedPath = parsePathParts(path);
</script>

<div class="pathSummary">
    <div class="pathHeader">
        <span class="pathLabel">Path</span>
        {#if parsedPath.queryEntries.length > 0}
            <span class="pathCount">
                {parsedPath.queryEntries.length} param{parsedPath.queryEntries.length ===
                1
                    ? ""
                    : "s"}
            </span>
        {/if}
    </div>

    <div class="pathValue" title={parsedPath.pathname}>{parsedPath.pathname}</div>

    {#if parsedPath.queryEntries.length > 0}
        <div class="queryList">
            {#each parsedPath.queryEntries as entry (entry.id)}
                <span class="queryChip">
                    <span class="queryKey">{entry.key || "(empty)"}</span>
                    <span class="queryEquals">=</span>
                    <span class="queryValue">{entry.value || "\"\""}</span>
                </span>
            {/each}
        </div>
    {/if}

    {#if parsedPath.fragment}
        <div class="fragmentLine">
            <span class="fragmentLabel">Fragment</span>
            <span class="fragmentValue" title={`#${parsedPath.fragment}`}
                >#{parsedPath.fragment}</span
            >
        </div>
    {/if}
</div>

<style>
    .pathSummary {
        display: grid;
        gap: 8px;
        padding: 10px 12px;
        border-radius: 12px;
        border: 1px solid var(--line);
        background: rgba(255, 255, 255, 0.02);
        min-width: 0;
    }

    .pathHeader,
    .fragmentLine {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 10px;
        min-width: 0;
    }

    .pathLabel,
    .fragmentLabel {
        color: var(--muted);
        font-size: 10px;
        font-weight: 900;
        letter-spacing: 0.12em;
        text-transform: uppercase;
    }

    .pathCount {
        color: var(--muted);
        font-size: 10px;
        font-weight: 700;
    }

    .pathValue,
    .fragmentValue {
        color: var(--text);
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
        font-size: 12px;
        line-height: 1.45;
        min-width: 0;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .pathValue {
        color: #dbeafe;
        font-weight: 700;
    }

    .queryList {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
    }

    .queryChip {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        min-width: 0;
        max-width: 100%;
        padding: 4px 8px;
        border-radius: 999px;
        border: 1px solid var(--line);
        background: var(--surface-muted);
        font-size: 10px;
        line-height: 1.3;
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
            "Liberation Mono", "Courier New", monospace;
    }

    .queryKey {
        color: #7dd3fc;
        font-weight: 700;
    }

    .queryEquals {
        color: var(--muted);
    }

    .queryValue {
        color: #86efac;
        word-break: break-word;
    }
</style>
