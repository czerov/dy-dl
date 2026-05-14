const state = {
  config: null,
  status: null,
  checks: [],
  downloads: [],
  logs: "",
  cookies: null,
  discovery: {
    result: null,
    selected: new Set(),
    filter: "all",
    sourceName: "",
  },
  activeView: "dashboard",
};

const viewTitles = {
  dashboard: "总览",
  users: "用户",
  discover: "发现",
  settings: "配置",
  history: "历史",
  logs: "日志",
};

const qualityOptions = ["best", "1080", "720", "480"];
const discoverTypeLabels = {
  all: "全部",
  work: "作品",
  collection: "合集",
  series: "短剧",
};

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
  document.getElementById("discover-button").addEventListener("click", discoverContent);
  document.getElementById("copy-collector-button").addEventListener("click", copyCollectorScript);
  document.getElementById("import-discovery-button").addEventListener("click", importDiscoveryContent);
  document.getElementById("download-selected-button").addEventListener("click", downloadSelectedDiscovery);
  document.getElementById("discover-user-select").addEventListener("change", applyDiscoverUser);
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
  renderDiscovery();
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
          ${field(`主页 URL`, `<input data-user-field="url" value="${escapeAttr(user.url)}" placeholder="抖音用户主页或单条视频链接，抖音号不是链接" />`)}
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

async function discoverContent() {
  const input = document.getElementById("discover-url-input");
  const sourceURL = input.value.trim();
  if (!sourceURL) {
    showToast("URL 为空");
    return;
  }

  try {
    showToast("正在获取列表");
    const result = await api("/api/discover", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url: sourceURL }),
    });
    state.discovery.result = result;
    state.discovery.selected = new Set();
    state.discovery.filter = "all";
    state.discovery.sourceName = currentDiscoverUserName() || "selected";
    renderDiscovery();
    showToast(`已获取 ${result.items.length} 个内容`);
  } catch (error) {
    showToast(error.message);
  }
}

async function copyCollectorScript() {
  const script = buildCollectorScript();
  const output = document.getElementById("collector-script");
  output.value = script;
  try {
    await copyText(script, output);
    showToast("采集脚本已复制");
  } catch (error) {
    showToast(error.message);
  }
}

async function importDiscoveryContent() {
  const input = document.getElementById("discover-import-input");
  const content = input.value.trim();
  if (!content) {
    showToast("采集结果为空");
    return;
  }

  try {
    const result = await api("/api/discover/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        url: document.getElementById("discover-url-input").value.trim(),
        content,
      }),
    });
    state.discovery.result = mergeDiscoveryResults(state.discovery.result, result);
    state.discovery.selected = new Set();
    state.discovery.filter = "all";
    state.discovery.sourceName = currentDiscoverUserName() || "selected";
    input.value = "";
    renderDiscovery();
    showToast(`已导入 ${result.items.length} 个内容`);
  } catch (error) {
    showToast(error.message);
  }
}

async function downloadSelectedDiscovery() {
  const result = state.discovery.result;
  if (!result || !result.items || !result.items.length) {
    showToast("没有可下载内容");
    return;
  }

  const urls = [...state.discovery.selected]
    .map((index) => result.items[Number(index)])
    .filter(Boolean)
    .map((item) => item.url);
  const uniqueURLs = [...new Set(urls)];
  if (!uniqueURLs.length) {
    showToast("先勾选要下载的内容");
    return;
  }

  try {
    await api("/api/discover/download", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        user_name: state.discovery.sourceName || currentDiscoverUserName() || "selected",
        quality: document.getElementById("discover-quality-select").value,
        save_dir: document.getElementById("discover-save-dir-input").value.trim(),
        urls: uniqueURLs,
      }),
    });
    showToast("选中内容已开始下载");
    await loadStatus();
    renderStatus();
  } catch (error) {
    showToast(error.message);
  }
}

function renderDiscovery() {
  renderDiscoveryForm();
  renderDiscoveryCollector();
  renderDiscoveryFilters();
  renderDiscoveryItems();
}

function renderDiscoveryForm() {
  const sourceSelect = document.getElementById("discover-user-select");
  const qualitySelectElement = document.getElementById("discover-quality-select");
  if (!sourceSelect || !qualitySelectElement) return;

  const previousSource = sourceSelect.value;
  const users = (state.config && state.config.users) || [];
  sourceSelect.innerHTML = [
    `<option value="">自定义</option>`,
    ...users.map((user, index) => {
      const label = user.name || user.url || `用户 ${index + 1}`;
      return `<option value="${index}">${escapeHTML(label)}</option>`;
    }),
  ].join("");
  if ([...sourceSelect.options].some((option) => option.value === previousSource)) {
    sourceSelect.value = previousSource;
  }

  const previousQuality = qualitySelectElement.value || "1080";
  qualitySelectElement.innerHTML = qualityOptions
    .map((option) => `<option value="${option}">${option}</option>`)
    .join("");
  qualitySelectElement.value = qualityOptions.includes(previousQuality) ? previousQuality : "1080";
}

function renderDiscoveryCollector() {
  const script = document.getElementById("collector-script");
  if (script) {
    script.value = buildCollectorScript();
  }
}

function renderDiscoveryFilters() {
  const filter = document.getElementById("discover-type-filter");
  if (!filter) return;
  const items = (state.discovery.result && state.discovery.result.items) || [];
  const counts = items.reduce(
    (acc, item) => {
      acc.all += 1;
      acc[item.type] = (acc[item.type] || 0) + 1;
      return acc;
    },
    { all: 0 },
  );

  const buttons = ["all", "work", "collection", "series"]
    .map((type) => {
      const active = state.discovery.filter === type ? "active" : "";
      return `<button class="filter-button ${active}" type="button" data-discover-filter="${type}">${discoverTypeLabels[type]} ${counts[type] || 0}</button>`;
    })
    .join("");
  filter.innerHTML = `${buttons}<span class="filter-spacer"></span><button class="ghost-button" type="button" id="discover-select-all">全选</button><button class="ghost-button" type="button" id="discover-clear-selection">清空</button>`;

  filter.querySelectorAll("[data-discover-filter]").forEach((button) => {
    button.addEventListener("click", () => {
      state.discovery.filter = button.dataset.discoverFilter;
      renderDiscoveryFilters();
      renderDiscoveryItems();
    });
  });
  document.getElementById("discover-select-all").addEventListener("click", () => {
    filteredDiscoveryEntries().forEach(({ index }) => state.discovery.selected.add(String(index)));
    renderDiscoveryItems();
  });
  document.getElementById("discover-clear-selection").addEventListener("click", () => {
    state.discovery.selected.clear();
    renderDiscoveryItems();
  });
}

function renderDiscoveryItems() {
  const list = document.getElementById("discover-list");
  if (!list) return;
  const result = state.discovery.result;
  if (!result) {
    list.innerHTML = '<div class="empty">暂无内容</div>';
    return;
  }

  const entries = filteredDiscoveryEntries();
  if (!entries.length) {
    list.innerHTML = '<div class="empty">当前类型暂无内容</div>';
    return;
  }

  list.innerHTML = entries
    .map(({ item, index }) => {
      const selected = state.discovery.selected.has(String(index)) ? "checked" : "";
      const title = item.title || `${discoverTypeLabels[item.type] || item.type} ${item.id}`;
      return `
        <label class="discover-row">
          <input type="checkbox" data-discover-select="${index}" ${selected} />
          <span class="discover-kind">${discoverTypeLabels[item.type] || item.type}</span>
          <span>
            <strong>${escapeHTML(title)}</strong>
            <span class="muted">${escapeHTML(item.url)}</span>
          </span>
        </label>
      `;
    })
    .join("");

  list.querySelectorAll("[data-discover-select]").forEach((input) => {
    input.addEventListener("change", () => {
      const index = input.dataset.discoverSelect;
      if (input.checked) {
        state.discovery.selected.add(index);
      } else {
        state.discovery.selected.delete(index);
      }
    });
  });
}

function filteredDiscoveryEntries() {
  const items = (state.discovery.result && state.discovery.result.items) || [];
  return items
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => state.discovery.filter === "all" || item.type === state.discovery.filter);
}

function applyDiscoverUser() {
  const user = selectedDiscoverUser();
  if (!user) return;
  document.getElementById("discover-url-input").value = user.url || "";
  document.getElementById("discover-quality-select").value = user.quality || "1080";
  document.getElementById("discover-save-dir-input").value = user.save_dir || "";
  state.discovery.sourceName = user.name || "selected";
}

function selectedDiscoverUser() {
  const select = document.getElementById("discover-user-select");
  if (!select || select.value === "" || !state.config || !state.config.users) return null;
  return state.config.users[Number(select.value)] || null;
}

function currentDiscoverUserName() {
  const user = selectedDiscoverUser();
  return user && user.name ? user.name : "";
}

function mergeDiscoveryResults(current, next) {
  const items = current && Array.isArray(current.items) ? current.items.slice() : [];
  const seen = new Map();
  items.forEach((item, index) => {
    seen.set(discoveryItemKey(item), index);
  });

  ((next && next.items) || []).forEach((item) => {
    const key = discoveryItemKey(item);
    if (!key) return;
    if (seen.has(key)) {
      const existing = items[seen.get(key)];
      if (!existing.title && item.title) existing.title = item.title;
      if (!existing.url && item.url) existing.url = item.url;
      return;
    }
    seen.set(key, items.length);
    items.push(item);
  });

  return {
    source_url: (next && next.source_url) || (current && current.source_url) || "",
    items,
  };
}

function discoveryItemKey(item) {
  if (!item) return "";
  return `${item.type || "unknown"}:${item.id || item.url || ""}`;
}

function buildCollectorScript() {
  return String.raw`(async () => {
  const bag = new Map();
  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const cleanTitle = (value) => String(value || "").replace(/\s+/g, " ").trim().slice(0, 160);
  const normalizeURL = (href) => {
    try {
      const url = new URL(href, location.origin);
      url.hash = "";
      return url.href;
    } catch {
      return "";
    }
  };
  const inferType = (url) => {
    try {
      const path = new URL(url).pathname;
      if (/\/video\/\d{10,}/.test(path)) return "work";
      if (/\/(?:collection|mix\/detail)\/\d{10,}/.test(path)) return "collection";
      if (/\/(?:series|playlet)\/\d{10,}/.test(path)) return "series";
    } catch {}
    return "";
  };
  const add = (href, title = "") => {
    const url = normalizeURL(href);
    const type = inferType(url);
    if (!type) return;
    const match = url.match(/\/(?:video|collection|series|playlet)\/(\d{10,})|\/mix\/detail\/(\d{10,})/);
    const id = match && (match[1] || match[2]);
    if (!id) return;
    const key = type + ":" + id;
    const current = bag.get(key);
    if (!current || (!current.title && title)) {
      bag.set(key, { type, id, title: cleanTitle(title), url });
    }
  };
  const collect = () => {
    document.querySelectorAll("a[href]").forEach((node) => {
      add(node.getAttribute("href"), node.innerText || node.title || node.getAttribute("aria-label") || "");
    });
    const html = document.documentElement.innerHTML.replaceAll("\\u002F", "/").replaceAll("\\/", "/");
    [
      [/\/video\/(\d{10,})/g, "https://www.douyin.com/video/"],
      [/\/(?:collection|mix\/detail)\/(\d{10,})/g, "https://www.douyin.com/collection/"],
      [/\/(?:series|playlet)\/(\d{10,})/g, "https://www.douyin.com/series/"],
    ].forEach(([pattern, prefix]) => {
      for (const match of html.matchAll(pattern)) add(prefix + match[1]);
    });
  };
  collect();
  let lastHeight = 0;
  let stable = 0;
  for (let step = 0; step < 24 && stable < 3; step += 1) {
    window.scrollTo(0, document.documentElement.scrollHeight);
    await wait(650);
    collect();
    const height = document.documentElement.scrollHeight;
    stable = height === lastHeight ? stable + 1 : 0;
    lastHeight = height;
  }
  const output = JSON.stringify({ source_url: location.href, items: [...bag.values()] }, null, 2);
  const showOutput = (text, copied) => {
    let panel = document.getElementById("__dydl_collector_panel__");
    if (!panel) {
      panel = document.createElement("div");
      panel.id = "__dydl_collector_panel__";
      Object.assign(panel.style, {
        position: "fixed",
        top: "16px",
        right: "16px",
        zIndex: "2147483647",
        width: "520px",
        maxWidth: "calc(100vw - 32px)",
        padding: "12px",
        borderRadius: "8px",
        background: "#111827",
        color: "#fff",
        boxShadow: "0 16px 36px rgba(0,0,0,.35)",
        fontFamily: "system-ui, -apple-system, Segoe UI, sans-serif",
      });
      panel.innerHTML =
        '<div style="display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:8px">' +
        '<strong style="font-size:14px">douyin-nas-monitor 采集结果</strong>' +
        '<button type="button" style="border:0;border-radius:6px;padding:6px 10px;cursor:pointer">关闭</button>' +
        '</div>' +
        '<div data-dydl-state style="font-size:12px;color:#cbd5e1;margin-bottom:8px"></div>' +
        '<textarea spellcheck="false" style="width:100%;height:280px;box-sizing:border-box;border:1px solid #475569;border-radius:6px;padding:8px;background:#020617;color:#e5e7eb;font:12px/1.45 Consolas, monospace"></textarea>';
      panel.querySelector("button").addEventListener("click", () => panel.remove());
      document.body.appendChild(panel);
    }
    panel.querySelector("[data-dydl-state]").textContent = copied
      ? "已复制到剪贴板，也可从下面手动复制。"
      : "浏览器拒绝自动复制，请从下面手动复制后粘贴到管理台。";
    const textarea = panel.querySelector("textarea");
    textarea.value = text;
    textarea.focus();
    textarea.select();
  };
  let copied = false;
  try {
    if (typeof copy === "function") {
      await Promise.resolve(copy(output));
      copied = true;
    } else if (navigator.clipboard && window.isSecureContext && document.hasFocus()) {
      await navigator.clipboard.writeText(output);
      copied = true;
    }
  } catch (error) {
    console.warn("douyin-nas-monitor copy failed, use the result panel instead.", error);
  }
  window.__DYDL_DISCOVERY_RESULT__ = output;
  if (!copied) {
    console.log(output);
  }
  showOutput(output, copied);
  console.log("douyin-nas-monitor collected " + bag.size + " item(s)");
  return [...bag.values()];
})();`;
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

async function copyText(text, fallbackElement) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  fallbackElement.focus();
  fallbackElement.select();
  if (!document.execCommand("copy")) {
    throw new Error("复制失败，请手动复制");
  }
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
