import { describe, expect, it } from "vitest";
import type { Folder } from "../api/types";
import { buildFolderTree, flattenFolderTree } from "./folders";

function folder(
  id: string,
  name: string,
  parent_id: string | null = null,
  sort_order = 0,
): Folder {
  return {
    id,
    name,
    parent_id,
    sort_order,
    is_starred: false,
    note_count: 0,
    child_count: 0,
    depth: parent_id ? 2 : 1,
    created_at: "2026-06-22T00:00:00Z",
    updated_at: "2026-06-22T00:00:00Z",
  };
}

describe("folder tree helpers", () => {
  it("builds a sorted nested tree from a flat folder list", () => {
    const tree = buildFolderTree([
      folder("root-b", "Work", null, 2),
      folder("child-b", "Zeta", "root-a", 2),
      folder("root-a", "Projects", null, 1),
      folder("child-a", "Alpha", "root-a", 1),
    ]);

    expect(tree.map((item) => item.folder.name)).toEqual(["Projects", "Work"]);
    expect(tree[0].children.map((item) => item.folder.name)).toEqual([
      "Alpha",
      "Zeta",
    ]);
  });

  it("flattens nested folders with depth for menus", () => {
    const options = flattenFolderTree(
      buildFolderTree([
        folder("root", "Root"),
        folder("child", "Child", "root"),
        folder("grandchild", "Grandchild", "child"),
      ]),
    );

    expect(options).toEqual([
      { id: "root", name: "Root", depth: 1 },
      { id: "child", name: "Child", depth: 2 },
      { id: "grandchild", name: "Grandchild", depth: 3 },
    ]);
  });
});
