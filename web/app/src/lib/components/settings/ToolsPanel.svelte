<script lang="ts">
  import type { User } from "../../api/types";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";

  export let user: User | null = null;
  export let onClose: () => void = () => {};

  interface ToolRow {
    label: string;
    action: () => void | Promise<void>;
    danger?: boolean;
    adminOnly?: boolean;
  }

  const rows: ToolRow[] = [
    {
      label: "导入",
      action: () => notify("导入中心将在后续步骤接入", "info"),
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

<section class="tools-panel" aria-label="工具">
  <div class="tools-panel__header">
    <h2>工具</h2>
    <button type="button" aria-label="关闭工具面板" on:click={onClose}>×</button>
  </div>
  <div class="tools-list">
    {#each visibleRows as row}
      <button
        class:danger={row.danger}
        type="button"
        on:click={() => {
          void row.action();
          if (!row.danger) onClose();
        }}
      >
        <span>{row.label}</span>
        <span aria-hidden="true">›</span>
      </button>
    {/each}
  </div>
</section>
