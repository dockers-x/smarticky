import { getToken } from "../api/client";

const attachmentDownloadPattern = /^\/api\/attachments\/\d+\/download(?:[?#].*)?$/;
export const protectedImagePlaceholderSrc =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==";

interface ProtectedImageRuntimeOptions {
  contentKey?: string;
}

function sameOriginPath(value: string): string {
  try {
    const url = new URL(value, window.location.origin);
    if (url.origin !== window.location.origin) return value;
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {
    return value;
  }
}

export function isProtectedAttachmentURL(value: string): boolean {
  return attachmentDownloadPattern.test(sameOriginPath(value));
}

export async function fetchProtectedImageObjectURL(value: string): Promise<string> {
  if (!isProtectedAttachmentURL(value)) return value;

  const token = getToken();
  const headers = new Headers();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(sameOriginPath(value), { headers });
  if (!response.ok) {
    throw new Error(`Image download failed: ${response.status}`);
  }

  return URL.createObjectURL(await response.blob());
}

export function protectedImageRuntime(
  node: HTMLElement,
  options: ProtectedImageRuntimeOptions = {},
): { update: (nextOptions?: ProtectedImageRuntimeOptions) => void; destroy: () => void } {
  const objectURLs = new Set<string>();
  let generation = 0;
  let activeContentKey = options.contentKey ?? "";

  function revokeObjectURLs(): void {
    for (const objectURL of objectURLs) {
      URL.revokeObjectURL(objectURL);
    }
    objectURLs.clear();
  }

  function loadImages(): void {
    const loadGeneration = ++generation;
    revokeObjectURLs();

    for (const image of node.querySelectorAll<HTMLImageElement>("img[data-auth-image='true']")) {
      const source = image.dataset.authSrc || "";
      if (!source) continue;

      image.classList.remove("markdown-image--failed");
      delete image.dataset.authImageLoaded;
      image.dataset.authImageLoading = "true";
      if (isProtectedAttachmentURL(source)) {
        image.src = protectedImagePlaceholderSrc;
      }

      void fetchProtectedImageObjectURL(source)
        .then((objectURL) => {
          if (loadGeneration !== generation) {
            if (objectURL.startsWith("blob:")) URL.revokeObjectURL(objectURL);
            return;
          }
          if (objectURL.startsWith("blob:")) objectURLs.add(objectURL);
          image.src = objectURL;
          image.dataset.authImageLoaded = "true";
        })
        .catch(() => {
          if (loadGeneration === generation) image.classList.add("markdown-image--failed");
        })
        .finally(() => {
          if (loadGeneration === generation) delete image.dataset.authImageLoading;
        });
    }
  }

  loadImages();

  return {
    update(nextOptions: ProtectedImageRuntimeOptions = {}) {
      const nextContentKey = nextOptions.contentKey ?? "";
      if (nextContentKey === activeContentKey) return;
      activeContentKey = nextContentKey;
      loadImages();
    },
    destroy() {
      generation++;
      revokeObjectURLs();
    },
  };
}
