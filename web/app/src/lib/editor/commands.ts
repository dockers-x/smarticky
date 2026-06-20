import { EditorSelection } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { t } from "../stores/preferences";

export function wrapSelection(
  view: EditorView,
  before: string,
  after = before,
): void {
  const { state } = view;
  const transaction = state.changeByRange((range) => {
    const selected = state.doc.sliceString(range.from, range.to);
    const insert = `${before}${selected}${after}`;
    const anchor = range.from + before.length;
    const head = range.to + before.length;

    return {
      changes: { from: range.from, to: range.to, insert },
      range: range.empty
        ? EditorSelection.cursor(anchor)
        : EditorSelection.range(anchor, head),
    };
  });

  view.dispatch(transaction);
  view.focus();
}

export function prefixLine(view: EditorView, prefix: string): void {
  const line = view.state.doc.lineAt(view.state.selection.main.from);
  view.dispatch({
    changes: { from: line.from, insert: prefix },
    selection: EditorSelection.cursor(
      view.state.selection.main.from + prefix.length,
    ),
  });
  view.focus();
}

export function insertTask(view: EditorView): void {
  prefixLine(view, "- [ ] ");
}

export function insertImage(view: EditorView): void {
  const pos = view.state.selection.main.from;
  const imageMarkdown = `![${t("imageInsertAlt")}]()`;

  view.dispatch({
    changes: { from: pos, insert: imageMarkdown },
    selection: EditorSelection.cursor(pos + imageMarkdown.length - 1),
  });
  view.focus();
}
