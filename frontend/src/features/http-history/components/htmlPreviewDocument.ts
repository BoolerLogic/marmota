const PREVIEW_CONTENT_SECURITY_POLICY = [
    "default-src 'none'",
    "base-uri 'none'",
    "script-src 'none'",
    "script-src-attr 'none'",
    "script-src-elem 'none'",
    "style-src 'unsafe-inline'",
    "img-src data:",
    "font-src data:",
    "media-src 'none'",
    "connect-src 'none'",
    "object-src 'none'",
    "frame-src 'none'",
    "child-src 'none'",
    "worker-src 'none'",
    "manifest-src 'none'",
    "form-action 'none'",
    "navigate-to 'none'",
    "prefetch-src 'none'",
].join("; ");

const PREVIEW_GUARD_STYLES = `
*, *::before, *::after {
    animation: none !important;
    scroll-behavior: auto !important;
    transition: none !important;
}
`;

const BLOCKED_ELEMENT_SELECTOR = [
    "script",
    "iframe",
    "frame",
    "frameset",
    "object",
    "embed",
    "applet",
    "portal",
    "base",
    "link",
    "noscript",
    "audio",
    "video",
    "source",
    "track",
    "animate",
    "animateMotion",
    "animateTransform",
    "set",
    "discard",
].join(",");

const VISUAL_RESOURCE_ATTRIBUTES = new Set([
    "src",
    "poster",
    "background",
]);

const REMOVED_URL_ATTRIBUTES = new Set([
    "action",
    "archive",
    "cite",
    "classid",
    "codebase",
    "data",
    "formaction",
    "longdesc",
    "manifest",
    "ping",
    "profile",
    "srcset",
]);

const INERT_ELEMENT_SELECTOR = [
    "a",
    "area",
    "button",
    "input",
    "select",
    "textarea",
    "option",
    "optgroup",
    "details",
    "summary",
    "dialog",
    "[contenteditable]",
].join(",");

function isEmbeddedVisualResource(value: string): boolean {
    const normalized = value.trim().toLowerCase();
    return normalized.startsWith("data:");
}

function isInternalFragment(value: string): boolean {
    return value.trim().startsWith("#");
}

function mayKeepHref(
    element: Element,
    attributeName: string,
    value: string,
): boolean {
    const tagName = element.localName.toLowerCase();

    if (tagName === "a" || tagName === "area") {
        return false;
    }

    if (isInternalFragment(value)) {
        // Preserve SVG paint servers, symbols and masks without turning
        // foreign markup (for example MathML) into a navigation surface.
        return element.namespaceURI === "http://www.w3.org/2000/svg";
    }

    return (
        (tagName === "image" || tagName === "use") &&
        (attributeName === "href" || attributeName === "xlink:href") &&
        isEmbeddedVisualResource(value)
    );
}

function removeUnsafeElements(root: ParentNode): void {
    root
        .querySelectorAll(BLOCKED_ELEMENT_SELECTOR)
        .forEach((element) => element.remove());

    root.querySelectorAll("meta[http-equiv]").forEach((element) => {
        element.remove();
    });
}

function removeUnsafeAttributes(root: ParentNode): void {
    root.querySelectorAll("*").forEach((element) => {
        for (const attribute of Array.from(element.attributes)) {
            const attributeName = attribute.name.toLowerCase();

            if (
                attributeName.startsWith("on") ||
                attributeName === "srcdoc" ||
                attributeName === "autofocus" ||
                attributeName === "accesskey" ||
                attributeName === "contenteditable" ||
                attributeName === "tabindex" ||
                attributeName === "target" ||
                attributeName === "download" ||
                attributeName === "popovertarget" ||
                attributeName === "popovertargetaction"
            ) {
                element.removeAttribute(attribute.name);
                continue;
            }

            if (REMOVED_URL_ATTRIBUTES.has(attributeName)) {
                element.removeAttribute(attribute.name);
                continue;
            }

            if (
                VISUAL_RESOURCE_ATTRIBUTES.has(attributeName) &&
                !isEmbeddedVisualResource(attribute.value)
            ) {
                element.removeAttribute(attribute.name);
                continue;
            }

            if (
                (attributeName === "href" ||
                    attributeName === "xlink:href") &&
                !mayKeepHref(element, attributeName, attribute.value)
            ) {
                element.removeAttribute(attribute.name);
                continue;
            }

            if (
                attributeName === "shadowrootmode" ||
                attributeName === "shadowrootdelegatesfocus" ||
                attributeName === "shadowrootclonable" ||
                attributeName === "shadowrootserializable"
            ) {
                element.removeAttribute(attribute.name);
            }
        }

    });

    root
        .querySelectorAll(`form,${INERT_ELEMENT_SELECTOR}`)
        .forEach((element) => {
            element.setAttribute("inert", "");
            element.setAttribute("aria-disabled", "true");
            element.removeAttribute("tabindex");
            element.removeAttribute("autofocus");
        });
}

function escapeHtml(value: string): string {
    return value
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
}

function buildPreviewDocument(bodyHtml: string, extraStyles = ""): string {
    return `<!doctype html>
<html>
<head>
<meta http-equiv="Content-Security-Policy" content="${escapeHtml(
        PREVIEW_CONTENT_SECURITY_POLICY,
    )}">
<meta name="referrer" content="no-referrer">
<style>${PREVIEW_GUARD_STYLES}${extraStyles}</style>
</head>
<body>${bodyHtml}</body>
</html>`;
}

function buildTextFallback(htmlText: string): string {
    return buildPreviewDocument(
        `<pre>${escapeHtml(htmlText)}</pre>`,
        `
body { margin: 16px; color: #111827; background: #ffffff; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; }
`,
    );
}

/**
 * Produces a complete, inert document for an opaque-origin sandboxed iframe.
 *
 * Parsing happens inside an inert template so resource attributes cannot issue
 * requests before sanitization. The serialized fragment is then placed after
 * a CSP that is the first element in the preview head. The iframe sandbox and
 * CSP remain the browser-enforced security boundaries.
 */
export function buildSafeHtmlPreviewDocument(htmlText: string): string {
    if (typeof document === "undefined") {
        return buildTextFallback(htmlText);
    }

    try {
        const template = document.createElement("template");
        template.innerHTML = htmlText;

        removeUnsafeElements(template.content);
        removeUnsafeAttributes(template.content);

        return buildPreviewDocument(template.innerHTML);
    } catch {
        return buildTextFallback(htmlText);
    }
}
