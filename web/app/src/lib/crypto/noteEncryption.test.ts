import { describe, expect, it } from "vitest";
import { decryptNoteContent, encryptNoteContent } from "./noteEncryption";

describe("note encryption", () => {
  it("round-trips note content without exposing plaintext in ciphertext", async () => {
    const payload = await encryptNoteContent("private body", "correct horse");

    expect(payload.encrypted_content).not.toContain("private body");
    expect(payload.encryption_alg).toBe("AES-GCM");
    expect(payload.encryption_kdf).toMatch(/^PBKDF2-SHA-256:/);

    await expect(decryptNoteContent(payload, "correct horse")).resolves.toBe(
      "private body",
    );
  });

  it("rejects wrong passwords", async () => {
    const payload = await encryptNoteContent("private body", "correct horse");

    await expect(decryptNoteContent(payload, "wrong password")).rejects.toThrow(
      "Unable to decrypt note content.",
    );
  });
});
