const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();
const iterations = 310_000;

export interface EncryptedNotePayload {
  encrypted_content: string;
  encryption_alg: "AES-GCM";
  encryption_kdf: string;
  encryption_salt: string;
  encryption_nonce: string;
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function bufferSource(bytes: Uint8Array): ArrayBuffer {
  const buffer = new ArrayBuffer(bytes.byteLength);
  new Uint8Array(buffer).set(bytes);
  return buffer;
}

async function deriveKey(password: string, salt: Uint8Array): Promise<CryptoKey> {
  const baseKey = await crypto.subtle.importKey(
    "raw",
    textEncoder.encode(password),
    "PBKDF2",
    false,
    ["deriveKey"],
  );

  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      hash: "SHA-256",
      salt: bufferSource(salt),
      iterations,
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"],
  );
}

export async function encryptNoteContent(
  content: string,
  password: string,
): Promise<EncryptedNotePayload> {
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const key = await deriveKey(password, salt);
  const encrypted = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: bufferSource(nonce) },
    key,
    bufferSource(textEncoder.encode(content)),
  );

  return {
    encrypted_content: bytesToBase64(new Uint8Array(encrypted)),
    encryption_alg: "AES-GCM",
    encryption_kdf: `PBKDF2-SHA-256:${iterations}`,
    encryption_salt: bytesToBase64(salt),
    encryption_nonce: bytesToBase64(nonce),
  };
}

export async function decryptNoteContent(
  payload: EncryptedNotePayload,
  password: string,
): Promise<string> {
  try {
    const salt = base64ToBytes(payload.encryption_salt);
    const nonce = base64ToBytes(payload.encryption_nonce);
    const key = await deriveKey(password, salt);
    const decrypted = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: bufferSource(nonce) },
      key,
      bufferSource(base64ToBytes(payload.encrypted_content)),
    );
    return textDecoder.decode(decrypted);
  } catch {
    throw new Error("Unable to decrypt note content.");
  }
}
