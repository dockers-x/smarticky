export interface EditorDraftValue {
  title: string;
  content: string;
}

export type EditorDraftFields = Partial<EditorDraftValue>;
export type EditorSaveStatus = "idle" | "pending" | "saving" | "saved" | "error";

export function createEditorDraft(
  initial: EditorDraftValue,
  persist: (fields: EditorDraftFields) => Promise<unknown>,
  onStatus: (status: EditorSaveStatus) => void,
) {
  const current = { ...initial };
  const saved = { ...initial };
  let status: EditorSaveStatus = "saved";
  let inFlight: Promise<boolean> | null = null;

  function changes(): EditorDraftFields {
    const fields: EditorDraftFields = {};
    if (current.title !== saved.title) fields.title = current.title;
    if (current.content !== saved.content) fields.content = current.content;
    return fields;
  }

  function report(next: EditorSaveStatus): void {
    status = next;
    onStatus(next);
  }

  async function save(): Promise<boolean> {
    while (Object.keys(changes()).length > 0) {
      const fields = changes();
      report("saving");
      try {
        await persist(fields);
        Object.assign(saved, fields);
      } catch {
        report("error");
        return false;
      }
    }
    report("saved");
    return true;
  }

  return {
    get value(): EditorDraftValue {
      return { ...current };
    },
    get changes(): EditorDraftFields {
      return changes();
    },
    get dirty(): boolean {
      return Object.keys(changes()).length > 0;
    },
    get status(): EditorSaveStatus {
      return status;
    },
    update(fields: EditorDraftFields): void {
      Object.assign(current, fields);
      if (!inFlight) {
        report(Object.keys(changes()).length > 0 ? "pending" : "saved");
      }
    },
    accept(fields: EditorDraftFields): void {
      Object.assign(current, fields);
      Object.assign(saved, fields);
    },
    flush(): Promise<boolean> {
      if (!inFlight && Object.keys(changes()).length === 0) {
        report("saved");
        return Promise.resolve(true);
      }
      if (!inFlight) {
        inFlight = save().finally(() => {
          inFlight = null;
        });
      }
      return inFlight;
    },
  };
}

export type EditorDraft = ReturnType<typeof createEditorDraft>;
