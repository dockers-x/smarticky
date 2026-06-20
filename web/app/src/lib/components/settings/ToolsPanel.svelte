<script lang="ts">
  import type { ImportResult } from "../../api/imports";
  import type { User } from "../../api/types";
  import ImportCenter from "../import/ImportCenter.svelte";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { importsStore } from "../../stores/imports";
  import { notesStore } from "../../stores/notes";

  export let user: User | null = null;
  export let onClose: () => void = () => {};

  type ToolsView = "menu" | "import";

  interface ToolRow {
    label: string;
    action: () => void | Promise<void>;
    danger?: boolean;
    adminOnly?: boolean;
    keepOpen?: boolean;
  }

  let view: ToolsView = "menu";

  function openImport(): void {
    importsStore.reset();
    view = "import";
  }

  async function handleImported(result: ImportResult): Promise<void> {
    await notesStore.setFilter("all");
    await notesStore.setSearch("");
    notify(
      result.failed_count > 0 ? "导入完成，部分条目失败" : "导入完成",
      result.failed_count > 0 ? "info" : "success",
    );
  }

  const rows: ToolRow[] = [
    {
      label: "导入",
      action: openImport,
      keepOpen: true,
    },
    {
      label: "备份",
      action: () => notify("备份工具将在设置面板中接入", "info"),
    },
    {
      label: "字体管理",
      action: () => notify("字体管理将在设置面板中接入", "info"),
    },
    {
      label: "个人资料",
      action: () => notify("个人资料将在设置面板中接入", "info"),
    },
    {
      label: "用户管理",
      adminOnly: true,
      action: () => notify("用户管理将在设置面板中接入", "info"),
    },
    {
      label: "退出登录",
      danger: true,
      action: async () => {
        const confirmed = await confirmDialog({
          title: "退出登录",
          message: "确认退出当前账号？",
          confirmLabel: "退出",
          cancelLabel: "取消",
        });
        if (confirmed) authStore.logout();
      },
    },
  ];

  $: visibleRows = rows.filter((row) => !row.adminOnly || user?.role === "admin");
</script>

<section class="tools-panel" class:import-view={view === "import"} aria-label="工具">
  <div class="tools-panel__header">
    <h2>{view === "import" ? "导入" : "工具"}</h2>
    <button type="button" aria-label="关闭工具面板" on:click={onClose}>×</button>
  </div>
  {#if view === "import"}
    <ImportCenter onBack={() => (view = "menu")} onImported={handleImported} />
  {:else}
    <div class="tools-list">
      {#each visibleRows as row}
        <button
          class:danger={row.danger}
          type="button"
          on:click={() => {
            void row.action();
            if (!row.danger && !row.keepOpen) onClose();
          }}
        >
          <span>{row.label}</span>
          <span aria-hidden="true">›</span>
        </button>
      {/each}
    </div>
  {/if}
</section>
