const state = {
  config: null,
  status: null,
  checks: [],
  downloads: [],
  logs: "",
  cookies: null,
  activeView: "dashboard",
};

const viewTitles = {
  dashboard: "总览",
  users: "用户",
  settings: "配置",
  history: "历史",
  logs: "日志",
};

const qualityOptions = ["best", "1080", "720", "480"];

document.addEventListener("DOMContentLoaded", () => {
  bindNavigation();
  bindActions();
  refreshAll();
  window.setInterval(refreshLiveData, 5000);
});

function bindNavigation() {
  document.querySelectorAll("[data-view-button]").forEach((button) => {
    button.addEventListener("click", () => {
      setView(button.dataset.viewButton);
    });
  });
}

function bindActions() {
  document.getElementById("refresh-button").addEventListener("click", refreshAll);
  document.getElementById("run-button").addEventListener("click", runNow);
  document.getElementById("check-button").addEventListener("click", loadChecks);
  document.getElementById("add-user-button").addEventListener("click", addUser);
  document.getElementById("save-users-button").addEventListener("click", saveConfig);
  document.getElementById("save-settings-button").addEventListener("click", saveConfig);
  document.getElementById("save-cookie-button").addEventListener("click", saveCookies);
  document.getElementById("refresh-history-button").addEventListener("click", loadDownloads);
  document.getElementById("refresh-logs-button").addEventListener("click", loadLogs);
}

async function refreshAll() {
  try {
    await Promise.all([loadConfig(), loadStatus(), loadDownloads(), loadLogs(), loadCookies()]);
    await loadChecks();
    render();
  } catch (error) {
    showToast(error.message);
  }
}

async function refreshLiveData() {
  try {
    await Promise.all([loadStatus(), loadDownloads(), loadLogs()]);
    renderStatus();
    renderDownloads();
    renderLogs();
  } catch (error) {
    showToast(error.message);
  }
}

async function loadConfig() {
  const data = await api("/api/config");
  state.config = data.config;
}

async function loadStatus() {
  state.status = await api("/api/status");
}

async function loadChecks() {
  state.checks = (await api("/api/check")) || [];
  renderChecks();
}

async function loadDownloads() {
  state.downloads = (await api("/api/downloads?limit=100")) || [];
  renderDownloads();
}

async function loadLogs() {
  const data = await api("/api/logs?lines=240");
  state.logs = data.text || "";
  renderLogs();
}

async function loadCookies() {
  state.cookies = await api("/api/cookies");
  renderCookies();
}

async function runNow() {
  try {
    await api("/api/run", { method: "POST" });
    showToast("下载任务已启动");
    await loadStatus();
    renderStatus();
  } catch (error) {
    showToast(error.message);
  }
}

async function saveCookies() {
  const input = document.getElementById("cookie-input");
  const content = input.value.trim();
  if (!content) {
    showToast("CK 内容为空");
    return;
  }

  try {
    state.cookies = await api("/api/cookies", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    });
    input.value = "";
    showToast("CK 已保存");
    renderCookies();
    await Promise.all([loadChecks(), loadLogs()]);
  } catch (error) {
    showToast(error.message);
  }
}

async function saveConfig() {
  try {
    collectSettings();
    collectUsers();
    const data = await api("/api/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ config: state.config }),
    });
    state.config = data.config;
    showToast("配置已保存");
    render();
  } catch (error) {
    showToast(error.message);
  }
}

async function api(path, options = {}) {
  const response = await fetch(path, options);
  const text = await response.text();
  let data = null;
  if (text) {
    data = JSON.parse(text);
  }
  if (!response.ok) {
    throw new Error((data && data.error) || response.statusText);
  }
  return data;
}

function render() {
  renderStatus();
  renderChecks();
  renderUsers();
  renderSettings();
  renderCookies();
  renderDownloads();
  renderLogs();
}

function renderStatus() {
  if (!state.status) return;

  const status = state.status;
  const runningText = status.running ? "运行中" : "空闲";
  const runState = document.getElementById("run-state");
  runState.textContent = runningText;
  runState.className = `status-pill ${status.running ? "running" : "ready"}`;

  document.getElementById("metric-running").textContent = runningText;
  document.getElementById("metric-users").textContent =
    `${status.users_enabled}/${status.users_total}`;
  document.getElementById("metric-mode").textContent = status.mode || "-";
  document.getElementById("metric-finished").textContent =
    formatTime(status.last_finished_at) || "-";

  const runButton = document.getElementById("run-button");
  runButton.disabled = status.running;
  runButton.textContent = status.running ? "运行中" : "立即运行";
}

function renderChecks() {
  const list = document.getElementById("check-list");
  if (!list) return;
  state.checks = state.checks || [];
  if (!state.checks.length) {
    list.innerHTML = '<div class="empty">暂无检查结果</div>';
    return;
  }
  list.innerHTML = state.checks
    .map((item) => {
      const ok = item.ok;
      return `
        <div class="check-item">
          <span class="check-status ${ok ? "ok" : "fail"}">${ok ? "OK" : "FAIL"}</span>
          <div>
            <strong>${escapeHTML(item.name)}</strong>
            <div class="muted">${escapeHTML(item.message || "")}</div>
          </div>
        </div>
      `;
    })
    .join("");
}

function renderUsers() {
  const list = document.getElementById("user-list");
  if (!list || !state.config) return;
  state.config.users = state.config.users || [];
  if (!state.config.users.length) {
    list.innerHTML = '<div class="empty">暂无用户</div>';
    return;
  }

  list.innerHTML = state.config.users
    .map((user, index) => {
      const quality = user.quality || "1080";
      return `
        <div class="user-row" data-user-index="${index}">
          ${field(`名称`, `<input data-user-field="name" value="${escapeAttr(user.name)}" />`)}
          ${field(`主页 URL`, `<input data-user-field="url" value="${escapeAttr(user.url)}" />`)}
          ${field(`清晰度`, qualitySelect(quality))}
          ${field(`保存目录`, `<input data-user-field="save_dir" value="${escapeAttr(user.save_dir || "")}" placeholder="留空用默认目录" />`)}
          ${field(`启用`, `<input data-user-field="enabled" type="checkbox" ${user.enabled ? "checked" : ""} />`)}
          <button class="danger-button" type="button" data-remove-user="${index}">删除</button>
        </div>
      `;
    })
    .join("");

  list.querySelectorAll("[data-remove-user]").forEach((button) => {
    button.addEventListener("click", () => {
      state.config.users.splice(Number(button.dataset.removeUser), 1);
      renderUsers();
    });
  });
}

function renderSettings() {
  const form = document.getElementById("settings-form");
  if (!form || !state.config) return;
  const { app, download, notify } = state.config;
  form.innerHTML = `
    ${field("运行模式", `
      <select data-app-field="mode">
        <option value="once" ${app.mode === "once" ? "selected" : ""}>once</option>
        <option value="daemon" ${app.mode === "daemon" ? "selected" : ""}>daemon</option>
      </select>
    `)}
    ${field("检测间隔分钟", `<input data-app-field="interval_minutes" type="number" min="1" value="${app.interval_minutes}" />`)}
    ${field("用户间隔秒", `<input data-app-field="sleep_between_users_seconds" type="number" min="0" value="${app.sleep_between_users_seconds}" />`)}
    ${field("日志文件", `<input data-app-field="log_file" value="${escapeAttr(app.log_file)}" />`, true)}
    ${field("数据库", `<input data-app-field="database" value="${escapeAttr(app.database)}" />`, true)}
    ${field("Cookie 文件", `<input data-app-field="cookies_file" value="${escapeAttr(app.cookies_file)}" />`, true)}
    ${field("归档文件", `<input data-app-field="archive_file" value="${escapeAttr(app.archive_file)}" />`, true)}
    ${field("默认下载目录", `<input data-app-field="default_save_dir" value="${escapeAttr(app.default_save_dir)}" />`, true)}
    ${field("yt-dlp 路径", `<input data-app-field="yt_dlp_path" value="${escapeAttr(app.yt_dlp_path)}" />`)}
    ${field("超时秒", `<input data-app-field="timeout_seconds" type="number" min="1" value="${app.timeout_seconds}" />`)}
    ${field("重试次数", `<input data-download-field="retries" type="number" min="0" value="${download.retries}" />`)}
    ${field("合并格式", `<input data-download-field="merge_output_format" value="${escapeAttr(download.merge_output_format)}" />`)}
    ${field("输出模板", `<input data-download-field="output_template" value="${escapeAttr(download.output_template)}" />`, true)}
    ${field("通知开关", `<input data-notify-field="enabled" type="checkbox" ${notify.enabled ? "checked" : ""} />`)}
    ${field("通知类型", `<input data-notify-field="type" value="${escapeAttr(notify.type)}" />`)}
    ${field("Webhook URL", `<input data-notify-field="webhook_url" value="${escapeAttr(notify.webhook_url)}" />`, true)}
  `;
}

function renderCookies() {
  const status = document.getElementById("cookie-status");
  if (!status) return;
  const cookies = state.cookies;
  if (!cookies) {
    status.textContent = "未加载";
    return;
  }

  const stateText = cookies.exists ? "已配置" : "未配置";
  const sizeText = cookies.exists ? ` · ${formatBytes(cookies.size)}` : "";
  const timeText = cookies.updated_at ? ` · ${formatTime(cookies.updated_at)}` : "";
  status.textContent = `${stateText} · ${cookies.path || "-"}${sizeText}${timeText}`;
}

function renderDownloads() {
  const rows = document.getElementById("download-rows");
  if (!rows) return;
  state.downloads = state.downloads || [];
  if (!state.downloads.length) {
    rows.innerHTML = `<tr><td colspan="5" class="empty">暂无下载记录</td></tr>`;
    return;
  }
  rows.innerHTML = state.downloads
    .map((item) => {
      const title = item.title || item.video_id || "-";
      return `
        <tr>
          <td>${escapeHTML(item.user_name || "-")}</td>
          <td>
            <strong>${escapeHTML(title)}</strong>
            <div class="muted">${escapeHTML(item.file_path || item.error || "")}</div>
          </td>
          <td>${escapeHTML(item.quality || "-")}</td>
          <td class="status-text ${escapeAttr(item.status)}">${escapeHTML(item.status || "-")}</td>
          <td>${escapeHTML(item.updated_at || "-")}</td>
        </tr>
      `;
    })
    .join("");
}

function renderLogs() {
  const box = document.getElementById("log-box");
  if (!box) return;
  box.textContent = state.logs || "暂无日志";
}

function setView(view) {
  state.activeView = view;
  document.querySelectorAll(".view").forEach((element) => {
    element.classList.toggle("active", element.id === `view-${view}`);
  });
  document.querySelectorAll("[data-view-button]").forEach((button) => {
    button.classList.toggle("active", button.dataset.viewButton === view);
  });
  document.getElementById("view-title").textContent = viewTitles[view] || view;
}

function addUser() {
  if (!state.config) return;
  state.config.users.push({
    name: "",
    url: "",
    enabled: true,
    quality: "1080",
    save_dir: "",
  });
  renderUsers();
}

function collectUsers() {
  const rows = document.querySelectorAll("[data-user-index]");
  rows.forEach((row) => {
    const index = Number(row.dataset.userIndex);
    const user = state.config.users[index];
    row.querySelectorAll("[data-user-field]").forEach((input) => {
      const fieldName = input.dataset.userField;
      user[fieldName] = input.type === "checkbox" ? input.checked : input.value.trim();
    });
  });
}

function collectSettings() {
  if (!state.config) return;
  collectFields("[data-app-field]", state.config.app);
  collectFields("[data-download-field]", state.config.download);
  collectFields("[data-notify-field]", state.config.notify);
}

function collectFields(selector, target) {
  document.querySelectorAll(selector).forEach((input) => {
    const fieldName = input.dataset.appField || input.dataset.downloadField || input.dataset.notifyField;
    if (input.type === "checkbox") {
      target[fieldName] = input.checked;
      return;
    }
    if (input.type === "number") {
      target[fieldName] = Number(input.value);
      return;
    }
    target[fieldName] = input.value.trim();
  });
}

function field(label, control, wide = false) {
  return `
    <div class="field ${wide ? "wide" : ""}">
      <label>${label}</label>
      ${control}
    </div>
  `;
}

function qualitySelect(value) {
  return `
    <select data-user-field="quality">
      ${qualityOptions
        .map((option) => `<option value="${option}" ${option === value ? "selected" : ""}>${option}</option>`)
        .join("")}
    </select>
  `;
}

function formatTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatBytes(value) {
  const size = Number(value) || 0;
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function showToast(message) {
  const toast = document.getElementById("toast");
  toast.textContent = message;
  toast.classList.add("show");
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => toast.classList.remove("show"), 3200);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeAttr(value) {
  return escapeHTML(value);
}
