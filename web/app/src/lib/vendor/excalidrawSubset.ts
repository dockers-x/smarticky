export const Commands = {
  Subset: "SUBSET",
} as const;

function toUint8Array(value: ArrayBuffer | Uint8Array): Uint8Array {
  return value instanceof Uint8Array ? value : new Uint8Array(value);
}

export async function toBase64(value: ArrayBuffer | Uint8Array): Promise<string> {
  const bytes = toUint8Array(value);
  let binary = "";
  const batchSize = 0x8000;

  for (let index = 0; index < bytes.length; index += batchSize) {
    const chunk = bytes.subarray(index, index + batchSize);
    binary += String.fromCharCode(...chunk);
  }

  return `data:font/woff2;base64,${btoa(binary)}`;
}

export async function subsetToBinary(
  arrayBuffer: ArrayBuffer,
  _codePoints: Iterable<number>,
): Promise<ArrayBuffer> {
  return arrayBuffer.slice(0);
}

export async function subsetToBase64(
  arrayBuffer: ArrayBuffer,
  codePoints: Iterable<number>,
): Promise<string> {
  return toBase64(await subsetToBinary(arrayBuffer, codePoints));
}
