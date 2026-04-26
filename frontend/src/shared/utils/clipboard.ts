export async function copyTextToClipboard(text: string): Promise<void> {
    if (
        typeof navigator !== "undefined" &&
        typeof navigator.clipboard?.writeText === "function"
    ) {
        await navigator.clipboard.writeText(text);
        return;
    }

    if (typeof document === "undefined") {
        throw new Error("Clipboard API is not available");
    }

    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "true");
    textarea.style.position = "fixed";
    textarea.style.top = "-9999px";
    textarea.style.left = "-9999px";

    document.body.appendChild(textarea);
    textarea.select();
    textarea.setSelectionRange(0, text.length);

    try {
        const execCommand = (
            document as unknown as Record<string, unknown>
        )["execCommand"];
        const copied =
            typeof execCommand === "function"
                ? (
                      execCommand as (
                          this: Document,
                          commandId: string,
                      ) => boolean
                  ).call(document, "copy")
                : false;
        if (!copied) {
            throw new Error("Could not copy to clipboard");
        }
    } finally {
        document.body.removeChild(textarea);
    }
}
