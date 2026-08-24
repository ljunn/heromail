const state = {
  token: localStorage.getItem("heromail_token") || "",
  user: null,
  role: "user",
  view: "apply",
  services: [],
  orders: [],
  mailboxes: [],
  overview: null,
  selectedService: "github",
  currentOrder: null,
  polling: null,
  pagination: {},
  apiKeys: [],
  ledgers: [],
  paymentMethods: [],
  paymentOrders: [],
  webhookEndpoints: [],
  webhookDeliveries: [],
  pools: [],
  users: [],
  auditLogs: [],
  paymentProviders: [],
  version: null,
  health: null,
  adminTabs: { users: "accounts", payments: "orders", operations: "alerts" },
  orderFilters: { status: "", service: "", query: "" },
  userOrderFilters: { status: "", service: "", query: "" },
  ledgerUserID: "",
  busyAction: "",
  pageError: "",
  orderError: "",
  paymentError: "",
  loading: false
};

const iconPaths = {
  logo: `<path d="M4 7.5 12 13l8-5.5"/><rect x="3" y="5" width="18" height="14" rx="3"/><path d="M16.5 16.5 18 18l3-3"/>`,
  inbox: `<path d="M4 4h16v12H4z"/><path d="m4 5 8 6 8-6"/><path d="M8 20h8"/>`,
  clock: `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
  receipt: `<path d="M6 3h12v18l-3-2-3 2-3-2-3 2z"/><path d="M9 8h6M9 12h6M9 16h3"/>`,
  fileCode: `<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M10 12l-2 2 2 2M14 12l2 2-2 2"/>`,
  key: `<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8M16 7l2 2M14 9l2 2"/>`,
  webhook: `<path d="M8.5 6a4 4 0 1 1 7 2.7L12 15"/><path d="M15.5 18a4 4 0 1 1-4.7-5.9L16 12"/><path d="M5.5 14a4 4 0 1 1 2.6-5.7L12 15"/>`,
  chart: `<path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/>`,
  wallet: `<path d="M4 6h15a2 2 0 0 1 2 2v10H4a2 2 0 0 1-2-2V6a3 3 0 0 1 3-3h13"/><path d="M16 11h5v4h-5z"/>`,
  userCog: `<circle cx="10" cy="8" r="4"/><path d="M3 21a7 7 0 0 1 14 0M19 12v5M16.5 14.5h5"/>`,
  activity: `<path d="M3 12h4l2-6 4 12 2-6h6"/>`,
  dashboard: `<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>`,
  mail: `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/>`,
  database: `<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/>`,
  plug: `<path d="M8 3v5M16 3v5M6 8h12v2a6 6 0 0 1-6 6v5M8 21h8"/>`,
  globe: `<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18M12 3a15 15 0 0 0 0 18"/>`,
  scan: `<path d="M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4"/><path d="M8 12h8M12 8v8"/>`,
  route: `<circle cx="6" cy="18" r="2"/><circle cx="18" cy="6" r="2"/><path d="M8 18h3a3 3 0 0 0 3-3V9a3 3 0 0 1 3-3"/>`,
  users: `<circle cx="9" cy="8" r="3"/><circle cx="17" cy="9" r="2.5"/><path d="M3 20a6 6 0 0 1 12 0M14 15a5 5 0 0 1 7 4.5"/>`,
  ledger: `<path d="M5 4h14v16H5z"/><path d="M9 4v16M12 8h4M12 12h4M12 16h3"/>`,
  creditCard: `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 10h18M7 15h4"/>`,
  bell: `<path d="M6 9a6 6 0 0 1 12 0c0 7 3 7 3 7H3s3 0 3-7M10 20h4"/>`,
  audit: `<path d="M8 4h8M9 2h6v4H9z"/><path d="M6 4H4v17h16V4h-2M8 11h8M8 15h8"/>`,
  settings: `<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/>`,
  shieldUser: `<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6z"/><circle cx="12" cy="10" r="2"/><path d="M8.8 16a3.5 3.5 0 0 1 6.4 0"/>`,
  code: `<path d="m8 9-3 3 3 3M16 9l3 3-3 3M14 5l-4 14"/>`,
  sparkle: `<path d="m12 3 1.2 4.8L18 9l-4.8 1.2L12 15l-1.2-4.8L6 9l4.8-1.2zM18 16l.6 2.4L21 19l-2.4.6L18 22l-.6-2.4L15 19l2.4-.6z"/>`,
  message: `<path d="M4 5h16v11H9l-5 4z"/><path d="M8 10h8M8 13h5"/>`,
  send: `<path d="m3 11 18-8-8 18-2-8zM11 13l5-5"/>`
};

function icon(name, className = "") {
  return `<svg class="ui-icon ${className}" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${iconPaths[name] || iconPaths.globe}</svg>`;
}

const userNav = [["apply", "申请邮箱", "inbox"], ["current", "当前任务", "clock"], ["orders", "订单", "receipt"], ["balance", "余额", "wallet"], ["keys", "开发者", "key"], ["settings", "账户", "userCog"]];
const mobileUserNav = [["apply", "申请", "inbox"], ["current", "任务", "clock"], ["orders", "订单", "receipt"], ["balance", "余额", "wallet"]];

const adminNav = [
  ["运行", [["admin-overview", "运行概览", "dashboard"]]],
  ["资源管理", [["admin-mailboxes", "邮箱池", "database"]]],
  ["业务配置", [["admin-services", "目标平台", "globe"]]],
  ["业务运营", [["admin-orders", "注册订单", "receipt"], ["admin-users", "用户与余额", "users"], ["admin-payments", "支付管理", "creditCard"]]],
  ["系统运维", [["admin-operations", "运维中心", "activity"], ["admin-settings", "版本升级", "settings"]]]
];

const userRoutes = { apply: "/app", current: "/app/tasks", orders: "/app/orders", keys: "/app/developer/keys", webhooks: "/app/developer/webhooks", usage: "/app/usage", balance: "/app/wallet", settings: "/app/account" };
const adminRoutes = { "admin-overview": "/admin", "admin-mailboxes": "/admin/mailboxes", "admin-channels": "/admin/channels", "admin-services": "/admin/services", "admin-orders": "/admin/orders", "admin-users": "/admin/users", "admin-payments": "/admin/payments", "admin-operations": "/admin/operations", "admin-settings": "/admin/settings", "admin-account": "/admin/account" };

function routeView(role, pathname = location.pathname) {
  if (role === "admin") {
    const tab = new URLSearchParams(location.search).get("tab");
    if (pathname === "/admin/ledger") { state.adminTabs.users = "ledger"; return "admin-users"; }
    if (pathname === "/admin/alerts") { state.adminTabs.operations = "alerts"; return "admin-operations"; }
    if (pathname === "/admin/audit") { state.adminTabs.operations = "audit"; return "admin-operations"; }
    if (pathname === "/admin/users" && ["accounts", "ledger"].includes(tab)) state.adminTabs.users = tab;
    if (pathname === "/admin/payments" && ["orders", "providers"].includes(tab)) state.adminTabs.payments = tab;
    if (pathname === "/admin/operations" && ["alerts", "audit"].includes(tab)) state.adminTabs.operations = tab;
  }
  const routes = role === "admin" ? adminRoutes : userRoutes;
  return Object.entries(routes).find(([, path]) => path === pathname)?.[0] || (role === "admin" ? "admin-overview" : "apply");
}

function redirectToLogin() {
  const redirect = `${location.pathname}${location.search}`;
  location.replace(`/login?redirect=${encodeURIComponent(redirect)}`);
}

async function navigate(view, replace = false) {
  const routes = state.role === "admin" ? adminRoutes : userRoutes;
  const path = routes[view];
  if (!path) return;
  stopPolling();
  state.view = view;
  history[replace ? "replaceState" : "pushState"]({ view }, "", path);
  await refresh();
}

async function api(path, options = {}) {
  const headers = { ...(options.body instanceof FormData ? {} : { "Content-Type": "application/json" }), ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  const response = await fetch(path, { ...options, headers });
  const body = await response.json().catch(() => ({}));
  if (response.status === 401 && state.token) {
    state.token = ""; state.user = null; localStorage.removeItem("heromail_token"); redirectToLogin();
  }
  if (!response.ok) throw new Error(body.message || "请求失败");
  return body;
}

const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[char]));
const money = value => `¥${Number(value || 0).toFixed(2)}`;
const time = value => { const parsed = value ? new Date(value) : null; return parsed && Number.isFinite(parsed.getTime()) && parsed.getUTCFullYear() > 1 ? parsed.toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }) : "-"; };
const statusMap = { created: ["待分配", "orange"], allocating: ["分配中", "blue"], assigned: ["等待提交", "orange"], waiting_code: ["收码中", "blue"], code_received: ["已收码", "green"], completed: ["已完成", "green"], canceled: ["已取消", "red"], expired_refunded: ["已超时退款", "orange"], allocation_failed: ["分配失败", "red"], disputed: ["申诉中", "orange"], pending: ["待支付", "orange"], paid: ["已支付", "blue"], active: ["正常", "green"], disabled: ["已停用", "red"], available: ["可用", "green"], leased: ["租用中", "blue"], cooldown: ["冷却中", "orange"], auth_error: ["授权异常", "red"], blocked: ["已隔离", "red"], verified: ["已验证", "green"], pending_verification: ["待验证", "orange"], failed: ["失败", "red"], idle: ["等待任务", "blue"], backing_up: ["备份中", "orange"], queued: ["已排队", "orange"], updating: ["升级中", "orange"], success: ["成功", "green"] };
function connectionLabel(method) { return ({ auto: "自动判定", microsoft_oauth: "Graph OAuth", microsoft_graph: "Graph", imap: "IMAP" })[method] || method || "自动判定"; }
function providerLabel(provider) { return ({ outlook: "Outlook", outlook_de: "Outlook.de", hotmail: "Hotmail" })[provider] || provider || "未知"; }
function providerList(providers) { return (providers || []).map(providerLabel).join(" / ") || "未配置"; }
const statusChip = status => { const item = statusMap[status] || [status, "blue"]; return `<span class="chip ${item[1]}">${item[0]}</span>`; };

function toast(message) {
  const el = document.querySelector("#toast"); el.textContent = message; el.classList.add("show");
  clearTimeout(toast.timer); toast.timer = setTimeout(() => el.classList.remove("show"), 2600);
}

function renderNav() {
  document.querySelector("#role-label").textContent = state.role === "admin" ? "管理员端" : "用户端";
  document.querySelector("#role-action").textContent = state.role === "admin" ? "用户门户" : "管理后台";
  const roleSwitch = document.querySelector("#role-switch");
  roleSwitch.style.display = state.user?.role === "admin" ? "inline-flex" : "none";
  roleSwitch.href = state.role === "admin" ? "/app" : "/admin";
  document.querySelector(".avatar").dataset.view = state.role === "admin" ? "admin-account" : "settings";
  document.querySelector("#nav").innerHTML = adminNav.map(([title, items]) => `<div class="nav-group"><div class="nav-title">${title}</div>${items.map(([view, label, iconName]) => `<button class="nav-item ${state.view === view ? "active" : ""}" data-action="view" data-view="${view}" aria-label="${label}"><span class="nav-icon">${icon(iconName)}</span><span>${label}</span></button>`).join("")}</div>`).join("");
  document.querySelector("#user-nav").innerHTML = userNav.map(([view, label, iconName]) => `<button class="user-nav-item ${state.view === view ? "active" : ""}" data-action="view" data-view="${view}" aria-label="${label}">${icon(iconName)}<span>${label}</span></button>`).join("") + `<a class="user-nav-item" href="/docs">${icon("fileCode")}<span>文档</span></a>`;
  document.querySelector("#mobile-nav").innerHTML = mobileUserNav.map(([view, label, iconName]) => `<button class="mobile-nav-item ${state.view === view ? "active" : ""}" data-action="view" data-view="${view}" aria-label="${label}">${icon(iconName)}<span>${label}</span></button>`).join("");
  const labels = Object.fromEntries((state.role === "admin" ? adminNav.flatMap(([, items]) => items) : userNav).map(([view, label]) => [view, label]));
  document.querySelector("#mobile-title").textContent = labels[state.view] || (state.view === "admin-account" ? "管理员账户" : "工作台");
}

function stat(label, value, note = "") { return `<div class="stat"><div class="stat-label">${label}</div><div class="stat-value">${value}</div><div class="stat-note">${note}</div></div>`; }
function pageHead(title, subtitle, action = "") { return `<div class="page-head"><div><h1>${title}</h1><p>${subtitle}</p></div>${action ? `<div class="head-actions">${action}</div>` : ""}</div>`; }
function sectionTabs(group, current, items) { return `<div class="section-tabs" role="tablist">${items.map(([value, label, count]) => `<button role="tab" aria-selected="${current === value}" class="section-tab ${current === value ? "active" : ""}" data-action="admin-tab" data-group="${group}" data-tab="${value}">${label}${count === undefined ? "" : `<span>${count}</span>`}</button>`).join("")}</div>`; }

function serviceCard(service) {
  const selected = service.code === state.selectedService;
  const serviceIcons = { github: "code", openai: "sparkle", discord: "message", telegram: "send" };
  return `<button type="button" class="service-card ${selected ? "selected" : ""}" data-action="service" data-service="${esc(service.code)}" aria-pressed="${selected}"><span class="service-logo">${icon(serviceIcons[service.code] || "globe")}</span><span class="service-name">${esc(service.name)}</span><span class="service-desc">${esc(service.description)}</span>${selected ? `<span class="selected-mark">✓</span>` : ""}</button>`;
}

function selectedService() { return state.services.find(service => service.code === state.selectedService) || state.services[0] || { name: "目标平台", price: 0, ttl_seconds: 600, allowed_providers: [] }; }

function renderApplyServiceGrid() {
  return state.services.map(serviceCard).join("") || `<div class="empty portal-empty-service"><strong>暂时没有可用平台</strong><span>请稍后刷新，或联系管理员确认服务状态。</span><button class="ghost-btn" data-action="refresh">刷新平台</button></div>`;
}

function renderApplySummary(service = selectedService()) {
  const inventory = Number(service.available_mailboxes || 0); const ttlMinutes = Math.max(1, Math.round(Number(service.ttl_seconds || 600) / 60)); const providers = providerList(service.allowed_providers);
  const balance = Number(state.user?.balance || 0); const price = Number(service.price || 0); const enoughBalance = balance >= price;
  const submit = state.busy ? `<button class="primary-btn portal-submit" disabled>正在分配…</button>` : !inventory ? `<button class="ghost-btn portal-submit" disabled>当前无可用库存</button>` : !enoughBalance ? `<button class="primary-btn portal-submit" data-action="view" data-view="balance">余额不足，去充值</button>` : `<button class="primary-btn portal-submit" data-action="create">申请邮箱</button>`;
  return `${state.orderError ? `<div class="inline-error" role="alert">${icon("activity")}<span>${esc(state.orderError)}</span></div>` : ""}<dl class="config-list"><div class="config-row"><dt>目标平台</dt><dd>${esc(service.name)}</dd></div><div class="config-row"><dt>邮箱类型</dt><dd>${esc(providers)}</dd></div><div class="config-row"><dt>可用库存</dt><dd>${inventory} 个</dd></div><div class="config-row"><dt>任务有效期</dt><dd>${ttlMinutes} 分钟</dd></div><div class="config-row"><dt>本次费用</dt><dd>${money(price)}</dd></div><div class="config-row"><dt>扣款规则</dt><dd>分配失败不扣，超时自动退</dd></div></dl>${submit}<p class="portal-policy">点击申请后会预扣本次费用。验证码到账后再完成结算。</p>`;
}

function renderApplyTaskBody() {
  return state.currentOrder ? renderTask(state.currentOrder) : `<div class="portal-empty-task">${icon("clock")}<strong>暂无进行中的任务</strong><span>申请成功后，邮箱、倒计时和验证码会集中显示在这里。</span><button class="link-btn" data-action="view" data-view="orders">查看历史订单</button></div>`;
}

function renderApply() {
  const service = selectedService();
  const balance = Number(state.user?.balance || 0);
  return `<div id="apply-view"><section class="portal-intro"><div><span>邮箱申请</span><h1>选择平台，开始收码任务</h1><p>系统自动分配可用邮箱。你只会看到本次任务需要的邮箱地址和验证码。</p></div><div class="portal-intro-actions"><span id="apply-balance" class="portal-balance">余额 ${money(balance)}</span><button class="ghost-btn" data-action="view" data-view="orders">查看订单</button></div></section><section class="portal-service-section"><div class="portal-section-head"><div><h2>目标平台</h2><p>价格、库存和有效期由管理员统一配置</p></div><span id="apply-selected-service" class="portal-balance" aria-live="polite">已选择 ${esc(service.name)}</span></div><div id="apply-service-grid" class="service-grid">${renderApplyServiceGrid()}</div></section><div class="portal-layout"><section class="card portal-order-tool"><div class="card-head"><h2>申请摘要</h2><span class="muted">系统自动选择邮箱</span></div><div id="apply-summary" class="card-body">${renderApplySummary(service)}</div></section><section class="card task-card portal-task-tool"><div class="card-head"><h2>当前任务</h2><span id="apply-task-status">${state.currentOrder ? statusChip(state.currentOrder.status) : ""}</span></div><div id="apply-task-body" class="card-body">${renderApplyTaskBody()}</div></section></div><div id="apply-recent-orders">${renderRecentOrders()}</div></div>`;
}

function renderTask(order) {
  const status = order.status;
  const steps = [["assigned", "邮箱已分配"], ["waiting_code", "等待收码"], ["code_received", "验证码已收到"], ["completed", "任务完成"]];
  const index = { assigned: 0, waiting_code: 1, code_received: 2, completed: 3 }[status] ?? 0;
  const remain = Math.max(0, Math.floor((new Date(order.expires_at) - Date.now()) / 1000));
  const actionBusy = state.busyAction && state.busyAction !== "";
  const countdown = `${Math.floor(remain / 60)}:${String(remain % 60).padStart(2, "0")}`;
  const countdownMarkup = `<span data-order-countdown="${esc(order.id)}">${countdown}</span>`;
  const guidance = status === "assigned" ? `请将此邮箱填写到 ${esc(order.service_name)} 注册页面，提交注册后点击下方按钮。` : status === "waiting_code" ? `已开始接收 ${esc(order.service_name)} 的验证邮件，验证码到达后会自动显示。` : "";
  return `<div class="steps">${steps.map(([key, label], i) => `<div class="step ${i < index ? "done" : i === index ? "current" : ""}">${label}</div>`).join("")}</div><div class="task-mail"><code>${esc(order.mailbox_address)}</code><button class="link-btn" data-action="copy" data-copy="${esc(order.mailbox_address)}">复制邮箱</button></div>${guidance ? `<div class="notice">${guidance}</div>` : ""}${status === "assigned" ? `<div class="action-row"><button class="primary-btn" data-action="submit" data-order="${order.id}" ${actionBusy ? "disabled" : ""}>${state.busyAction === "submitted" ? "正在提交…" : "我已提交注册"}</button><button class="danger-btn" data-action="cancel" data-order="${order.id}" ${actionBusy ? "disabled" : ""}>取消并退款</button></div>` : ""}${status === "waiting_code" ? `<div class="notice warning">正在等待平台注册邮件，剩余 ${countdownMarkup}。</div>` : ""}${order.code ? `<div class="code-box"><div><div class="code-label">验证码（已收到）</div><div class="code">${esc(order.code)}</div></div><div class="timer">${countdownMarkup}</div></div><div class="notice success">验证码已提取成功，请在目标平台完成验证。</div>${status === "code_received" ? `<div class="action-row"><button class="primary-btn" data-action="complete" data-order="${order.id}" ${actionBusy ? "disabled" : ""}>${state.busyAction === "complete" ? "正在完成…" : "完成注册"}</button></div>` : ""}` : ""}`;
}

function renderRecentOrders() {
  const orders = state.orders.slice(0, 5);
  return `<div class="card recent-card"><div class="card-head"><h2>最近订单</h2><button class="link-btn" data-action="view" data-view="orders">查看全部订单 →</button></div><div class="table-wrap">${orders.length ? `<table><thead><tr><th>订单号</th><th>目标平台</th><th>状态</th><th>费用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${orders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.service_name)}</td><td>${statusChip(order.status)}</td><td>${money(order.price)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">查看详情</button></td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单，申请一个邮箱开始任务</div>`}</div></div>`;
}

function updateApplyView(regions = {}) {
  if (state.view !== "apply") return Promise.resolve();
  if (!document.querySelector("#apply-view")) return render();
  const all = regions.all === true;
  const service = selectedService();
  if (all || regions.services) document.querySelector("#apply-service-grid").innerHTML = renderApplyServiceGrid();
  if (all || regions.selection) {
    document.querySelectorAll("#apply-service-grid [data-service]").forEach(card => {
      const selected = card.dataset.service === state.selectedService;
      card.classList.toggle("selected", selected);
      card.setAttribute("aria-pressed", String(selected));
      const mark = card.querySelector(".selected-mark");
      if (selected && !mark) {
        const nextMark = document.createElement("span");
        nextMark.className = "selected-mark";
        nextMark.textContent = "✓";
        card.appendChild(nextMark);
      } else if (!selected) mark?.remove();
    });
  }
  const selectedLabel = document.querySelector("#apply-selected-service");
  if (selectedLabel) selectedLabel.textContent = `已选择 ${service.name}`;
  if (all || regions.summary) document.querySelector("#apply-summary").innerHTML = renderApplySummary(service);
  if (all || regions.task) {
    document.querySelector("#apply-task-status").innerHTML = state.currentOrder ? statusChip(state.currentOrder.status) : "";
    document.querySelector("#apply-task-body").innerHTML = renderApplyTaskBody();
  }
  if (all || regions.recent) document.querySelector("#apply-recent-orders").innerHTML = renderRecentOrders();
  const balance = document.querySelector("#apply-balance");
  if (balance) balance.textContent = `余额 ${money(state.user?.balance)}`;
  return Promise.resolve();
}

function updateTaskCountdown(order) {
  const remain = Math.max(0, Math.floor((new Date(order.expires_at) - Date.now()) / 1000));
  const countdown = `${Math.floor(remain / 60)}:${String(remain % 60).padStart(2, "0")}`;
  document.querySelectorAll("[data-order-countdown]").forEach(element => {
    if (element.dataset.orderCountdown === order.id) element.textContent = countdown;
  });
}

function renderOrders() {
  const filters = state.userOrderFilters; const total = state.pagination.orders?.total || 0;
  const statusOptions = [["", "全部状态"], ["assigned", "等待提交"], ["waiting_code", "收码中"], ["code_received", "已收码"], ["completed", "已完成"], ["canceled", "已取消"], ["expired_refunded", "已退款"]];
  return pageHead("订单记录", "按平台、状态或订单号查找历史任务。敏感邮箱和验证码只在订单详情中显示。", `<button class="primary-btn" data-action="view" data-view="apply">申请邮箱</button>`) + `<div class="stat-grid">${stat("筛选结果", total, "服务端分页")}${stat("当前任务", state.currentOrder ? 1 : 0, "正在处理")}${stat("当前页", state.orders.length, "本页记录")}</div><form class="filter-bar" data-form="user-order-filters"><select id="user-order-service" class="select" aria-label="目标平台"><option value="">全部平台</option>${state.services.map(service => `<option value="${esc(service.code)}" ${filters.service === service.code ? "selected" : ""}>${esc(service.name)}</option>`).join("")}</select><select id="user-order-status" class="select" aria-label="订单状态">${statusOptions.map(([value, label]) => `<option value="${value}" ${filters.status === value ? "selected" : ""}>${label}</option>`).join("")}</select><input id="user-order-query" class="search" value="${esc(filters.query)}" placeholder="订单号或邮箱"><button class="primary-btn" type="submit">查询</button>${filters.status || filters.service || filters.query ? `<button class="ghost-btn" type="button" data-action="reset-user-order-filters">清空</button>` : ""}</form><div class="card"><div class="table-wrap"><table><thead><tr><th>订单号</th><th>目标平台</th><th>状态</th><th>费用</th><th>有效期</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.orders.length ? state.orders.map(order => `<tr class="${state.currentOrder && state.currentOrder.id === order.id ? "selected" : ""}"><td>${esc(order.id)}</td><td>${esc(order.service_name)}</td><td>${statusChip(order.status)}</td><td>${money(order.price)}</td><td>${order.status === "completed" ? "—" : time(order.expires_at)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">查看详情</button></td></tr>`).join("") : `<tr><td colspan="7" class="empty">没有符合条件的订单，可清空筛选或申请新邮箱</td></tr>`}</tbody></table></div>${renderPager("orders")}</div>${state.currentOrder ? `<div class="card order-detail-card"><div class="card-head"><h2>订单详情 · ${esc(state.currentOrder.id)}</h2>${statusChip(state.currentOrder.status)}</div><div class="card-body"><div class="task-mail"><code>${esc(state.currentOrder.mailbox_address)}</code><button class="link-btn" data-action="copy" data-copy="${esc(state.currentOrder.mailbox_address)}">复制邮箱</button></div>${state.currentOrder.code ? `<div class="code-box"><div><div class="code-label">验证码</div><div class="code">${esc(state.currentOrder.code)}</div></div></div>` : ""}<div class="timeline">${[["创建订单", state.currentOrder.created_at], ["分配邮箱", state.currentOrder.assigned_at], ["用户已提交", state.currentOrder.submitted_at], ["收到验证码", state.currentOrder.code_received_at], ["完成结算", state.currentOrder.completed_at]].map(([label, value]) => `<div class="timeline-item ${value ? "done" : ""}"><span class="timeline-dot"></span><div><div class="timeline-title">${label}</div><div class="timeline-time">${time(value)}</div></div></div>`).join("")}</div><div class="notice">邮箱凭证和完整邮件内容不会提供。提交后未收到验证码会按规则自动退款。</div></div></div>` : ""}`;
}

function renderCurrentTaskBody() {
  return state.currentOrder ? renderTask(state.currentOrder) : `<div class="portal-empty-task">${icon("clock")}<strong>当前没有进行中的注册任务</strong><span>从申请邮箱开始，系统会在这里持续更新状态和验证码。</span><button class="primary-btn" data-action="view" data-view="apply">申请邮箱</button></div>`;
}

function renderCurrent() {
  return pageHead("当前任务", "正在进行的平台注册任务会在这里显示。", `<button class="ghost-btn" data-action="view" data-view="apply">申请新邮箱</button>`) + `<div class="card"><div id="current-task-body" class="card-body">${renderCurrentTaskBody()}</div></div>`;
}

function updateCurrentTaskView() {
  if (state.view !== "current") return Promise.resolve();
  const body = document.querySelector("#current-task-body");
  if (!body) return render();
  body.innerHTML = renderCurrentTaskBody();
  return Promise.resolve();
}

function renderAdminOverview() {
  const data = state.overview || {}; const services = state.services; const orders = state.orders.slice(0, 6);
  const maxInventory = Math.max(1, ...services.map(service => Number(service.available_mailboxes || 0)));
  return pageHead("运行概览", "邮箱库存、平台注册任务与收码服务的实时状态。", `<button class="ghost-btn" data-action="refresh">刷新数据</button>`) + `<div class="stat-grid">${stat("可分配邮箱", data.available_mailboxes ?? 0, "全局健康可用")}${stat("活跃租约", data.active_leases ?? 0, "正在进行的任务")}${stat("今日注册订单", data.today_orders ?? 0, "UTC 自然日")}${stat("收码成功率", `${Number(data.success_rate ?? 0).toFixed(2)}%`, "今日实时统计")}${stat("平均收码时间", `${Number(data.average_code_seconds ?? 0).toFixed(1)} 秒`, "今日实时统计")}</div><div class="admin-grid"><div class="card"><div class="card-head"><h2>目标平台库存</h2><button class="link-btn" data-action="view" data-view="admin-services">管理平台 →</button></div><div class="card-body"><div class="bar-list">${services.map(service => { const count = Number(service.available_mailboxes || 0); return `<div class="bar-row"><span>${esc(service.name)}</span><div class="bar-track"><div class="bar-fill" style="width:${Math.round(count / maxInventory * 100)}%"></div></div><strong>${count}</strong></div>`; }).join("") || `<div class="empty">暂无平台</div>`}</div></div></div><div class="card"><div class="card-head"><h2>邮箱渠道健康</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> 可用邮箱 <strong style="margin-left:auto">${data.available_mailboxes ?? 0}</strong></div><div class="status-item"><i class="status-dot ${data.auth_errors ? "orange" : ""}"></i> 授权异常 <strong style="margin-left:auto">${data.auth_errors ?? 0}</strong></div><div class="status-item"><i class="status-dot ${data.blocked_mailboxes ? "red" : ""}"></i> 已隔离 <strong style="margin-left:auto">${data.blocked_mailboxes ?? 0}</strong></div></div></div></div></div><div class="admin-grid"><div class="card"><div class="card-head"><h2>最近注册订单</h2><button class="link-btn" data-action="view" data-view="admin-orders">查看全部 →</button></div><div class="table-wrap">${orders.length ? `<table><thead><tr><th>订单号</th><th>用户</th><th>平台</th><th>邮箱</th><th>状态</th><th>有效期</th></tr></thead><tbody>${orders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.user_id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${order.status === "completed" ? "—" : time(order.expires_at)}</td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单</div>`}</div></div><div class="card"><div class="card-head"><h2>快捷操作</h2></div><div class="card-body"><div class="service-grid"><button class="service-card" data-action="view" data-view="admin-mailboxes"><span class="service-logo">${icon("mail")}</span><span class="service-name">邮箱资源</span><span class="service-desc">检查授权与库存</span></button><button class="service-card" data-action="view" data-view="admin-services"><span class="service-logo">${icon("globe")}</span><span class="service-name">目标平台</span><span class="service-desc">配置规则和价格</span></button></div></div></div></div>`;
}

async function verifyMailbox(id) { await api(`/api/v1/admin/mailboxes/${id}/verify`, { method: "POST" }); await refresh(); toast("邮箱连接验证成功"); }
async function showMailboxMessages(id, address, page = 1) {
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal mailbox-messages-modal"><div class="card-head"><div><h2>收件箱</h2><div class="modal-subtitle">${esc(address || "邮箱")} · 只读查看</div></div><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body mailbox-messages-body"><div class="portal-loading">正在读取收件箱…</div></div></div></div>`);
  try {
    const result = await api(`/api/v1/admin/mailboxes/${encodeURIComponent(id)}/messages?page=${page}&page_size=50`);
    const messages = result.data || [];
    const pagination = result.pagination || {};
    const body = document.querySelector(".mailbox-messages-body");
    if (!body) return;
    body.innerHTML = messages.length ? `<div class="mailbox-messages-meta">共 ${pagination.total || messages.length} 封，最多读取最近 1000 封</div><div class="mailbox-message-list">${messages.map(message => `<article class="mailbox-message"><div class="mailbox-message-head"><div><strong>${esc(message.subject || "无主题")}</strong><span>${esc(message.sender || "未知发件人")}</span></div><time>${time(message.received_at)}</time></div><pre>${esc(message.body || message.body_preview || "（无正文）")}</pre></article>`).join("")}</div>${pagination.total_pages > 1 ? `<div class="mailbox-message-pager"><button class="ghost-btn" data-action="mailbox-message-page" data-id="${esc(id)}" data-address="${esc(address || "邮箱")}" data-page="${pagination.page - 1}" ${pagination.page <= 1 ? "disabled" : ""}>上一页</button><span>第 ${pagination.page} / ${pagination.total_pages} 页</span><button class="ghost-btn" data-action="mailbox-message-page" data-id="${esc(id)}" data-address="${esc(address || "邮箱")}" data-page="${pagination.page + 1}" ${pagination.page >= pagination.total_pages ? "disabled" : ""}>下一页</button></div>` : ""}` : `<div class="empty">收件箱暂无邮件</div>`;
  } catch (error) {
    const body = document.querySelector(".mailbox-messages-body");
    if (body) body.innerHTML = `<div class="inline-error" role="alert">${icon("activity")}<span>${esc(error.message)}</span></div>`;
  }
}
function renderAdminServices() {
  return pageHead("目标平台", "在一处配置可用邮箱、邮件匹配、价格和任务有效期。", `<button class="primary-btn" data-action="edit-service">新建目标平台</button>`) + `<div class="card"><div class="table-wrap"><table><thead><tr><th>平台</th><th>邮箱类型</th><th>邮件匹配</th><th>库存</th><th>单价 / 有效期</th><th>状态</th><th>操作</th></tr></thead><tbody>${state.services.map(service => `<tr><td><strong>${esc(service.name)}</strong><div class="muted"><code>${esc(service.code)}</code> · ${esc(service.description)}</div></td><td>${esc(providerList(service.allowed_providers))}</td><td><div>${esc((service.sender_domains || []).join(", "))}</div><div class="muted">${esc((service.subject_keywords || []).join(", ") || "不限制主题")}</div></td><td><strong>${service.available_mailboxes ?? 0}</strong><div class="muted">租用 ${service.leased_mailboxes ?? 0} · 已使用 ${service.consumed_mailboxes ?? 0}</div></td><td>${money(service.price)}<div class="muted">${Math.round(service.ttl_seconds / 60)} 分钟</div></td><td>${service.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><div class="table-actions"><button class="link-btn" data-action="edit-service" data-id="${esc(service.id)}">编辑</button><button class="link-btn danger-text" data-action="delete-service" data-id="${esc(service.id)}">删除</button></div></td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无目标平台</td></tr>`}</tbody></table></div>${renderPager("admin-services")}</div>`;
}

function renderAdminOrders() {
  const total = state.pagination["admin-orders"]?.total || 0;
  const filters = state.orderFilters;
  const statusOptions = [["", "全部状态"], ["created", "待分配"], ["allocating", "分配中"], ["assigned", "等待提交"], ["waiting_code", "收码中"], ["code_received", "已收码"], ["completed", "已完成"], ["canceled", "已取消"], ["expired_refunded", "已退款"], ["allocation_failed", "分配失败"], ["disputed", "申诉中"]];
  const rows = state.orders.map(order => `<tr class="${state.currentOrder?.id === order.id ? "selected" : ""}"><td data-label="订单号"><button class="order-id-button" data-action="select-order" data-order="${esc(order.id)}">${esc(order.id)}</button></td><td data-label="用户"><strong>${esc(order.user_email || order.user_id)}</strong><span class="table-subline">${esc(order.user_id)}</span></td><td data-label="目标平台">${esc(order.service_name)}</td><td data-label="状态">${statusChip(order.status)}</td><td data-label="费用">${money(order.price)}</td><td data-label="创建时间">${time(order.created_at)}</td><td data-label="操作" class="row-action"><button class="icon-btn compact" data-action="select-order" data-order="${esc(order.id)}" title="查看订单详情">${icon("receipt")}</button></td></tr>`).join("");
  return pageHead("注册订单", "按状态、平台或关键字定位任务，打开右侧面板查看完整链路。", `<button class="ghost-btn" data-action="refresh">刷新</button>`) + `<div class="operation-strip"><div><span>筛选结果</span><strong>${total}</strong><small>服务端分页统计</small></div><p>订单状态、退款结果和邮箱占用由业务状态机维护，管理员在详情中核对事实。</p></div><form class="filter-bar admin-filter-bar" data-form="order-filters"><select id="admin-order-status" class="select" aria-label="订单状态">${statusOptions.map(([value, label]) => `<option value="${value}" ${filters.status === value ? "selected" : ""}>${label}</option>`).join("")}</select><select id="admin-order-service" class="select" aria-label="目标平台"><option value="">全部平台</option>${state.services.map(service => `<option value="${esc(service.code)}" ${filters.service === service.code ? "selected" : ""}>${esc(service.name)}</option>`).join("")}</select><input id="admin-order-query" class="search" aria-label="搜索订单" value="${esc(filters.query)}" placeholder="订单号、用户 ID 或邮箱"><button class="primary-btn" type="submit">查询</button>${filters.status || filters.service || filters.query ? `<button class="ghost-btn" type="button" data-action="reset-order-filters">清空</button>` : ""}</form><div class="card"><div class="table-wrap responsive-admin-table"><table class="order-table"><thead><tr><th>订单号</th><th>用户</th><th>目标平台</th><th>状态</th><th>费用</th><th>创建时间</th><th><span class="sr-only">操作</span></th></tr></thead><tbody>${rows || `<tr><td colspan="7" class="empty">没有符合条件的订单</td></tr>`}</tbody></table></div>${renderPager("admin-orders")}</div>${renderAdminOrderDrawer()}`;
}

function renderAdminOrderDrawer() {
  const order = state.currentOrder;
  if (!order || state.view !== "admin-orders") return "";
  const timeline = [["创建订单", order.created_at], ["分配邮箱", order.assigned_at], ["用户已提交", order.submitted_at], ["收到验证码", order.code_received_at], ["完成结算", order.completed_at]];
  return `<div class="detail-layer"><button class="detail-scrim" data-action="close-order-detail" aria-label="关闭订单详情"></button><aside class="detail-panel" aria-label="订单详情"><div class="detail-panel-head"><div><span class="detail-eyebrow">订单详情</span><h2>${esc(order.id)}</h2></div><button class="icon-btn" data-action="close-order-detail" title="关闭">×</button></div><div class="detail-panel-body"><div class="detail-status">${statusChip(order.status)}<span>${order.refunded ? "费用已退回" : "按订单状态结算"}</span></div><dl class="detail-list"><div><dt>用户</dt><dd>${esc(order.user_email || order.user_id)}<small>${esc(order.user_id)}</small></dd></div><div><dt>目标平台</dt><dd>${esc(order.service_name)}</dd></div><div><dt>分配邮箱</dt><dd>${esc(order.mailbox_address)}</dd></div><div><dt>验证码</dt><dd>${esc(order.code || "尚未收到")}</dd></div><div><dt>费用</dt><dd>${money(order.price)}</dd></div><div><dt>请求 ID</dt><dd>${esc(order.request_id || "—")}</dd></div><div><dt>失败原因</dt><dd class="${order.failure_reason ? "danger-text" : ""}">${esc(order.failure_reason || "无")}</dd></div></dl><section class="detail-section"><h3>状态时间线</h3><div class="timeline">${timeline.map(([label, value]) => `<div class="timeline-item ${value ? "done" : ""}"><span class="timeline-dot"></span><div><div class="timeline-title">${label}</div><div class="timeline-time">${time(value)}</div></div></div>`).join("")}</div></section></div></aside></div>`;
}

function renderPager(key) {
  const page = state.pagination[key]; if (!page || page.total_pages <= 1) return "";
  return `<div class="pager"><button class="ghost-btn" data-action="page" data-key="${key}" data-page="${page.page - 1}" ${page.page <= 1 ? "disabled" : ""}>上一页</button><span>第 ${page.page} / ${page.total_pages} 页 · 共 ${page.total} 条</span><button class="ghost-btn" data-action="page" data-key="${key}" data-page="${page.page + 1}" ${page.page >= page.total_pages ? "disabled" : ""}>下一页</button></div>`;
}

function renderAPIKeys() {
  return pageHead("开发者", "管理 API 密钥、Webhook 和接口文档。", `<div class="page-head-tabs"><button class="ghost-btn" data-action="view" data-view="webhooks">Webhook</button><button class="ghost-btn" data-action="view" data-view="docs">API 文档</button></div>`) + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>密钥列表</h2></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>前缀</th><th>权限</th><th>最后使用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.apiKeys.map(key => `<tr><td>${esc(key.name)}</td><td><code>${esc(key.prefix)}…</code></td><td>${esc(key.scopes.join(", "))}</td><td>${time(key.last_used_at)}</td><td>${time(key.created_at)}</td><td><button class="link-btn danger-text" data-action="revoke-key" data-id="${key.id}">吊销</button></td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无 API Key，创建一个给服务端调用</td></tr>`}</tbody></table></div>${renderPager("keys")}</div><div class="card"><div class="card-head"><h2>创建密钥</h2></div><div class="card-body form-grid"><label>密钥名称<input id="key-name" class="field" placeholder="生产环境"></label><label>权限范围<select id="key-scope" class="field"><option value="orders">订单读写</option><option value="read">仅订单读取</option></select></label><button class="primary-btn" data-action="create-key">创建 API Key</button><div class="notice">密钥明文只显示一次，服务端仅保存 SHA-256 哈希。</div></div></div></div>`;
}

function renderUsage() {
  return pageHead("用量与账单", "查看订单扣费、退款、支付充值和人工调整记录。") + `<div class="stat-grid">${stat("当前余额", money(state.user.balance), "可用额度")}${stat("流水数量", state.pagination.ledgers?.total || 0, "全部资金变动")}</div><div class="card"><div class="table-wrap"><table><thead><tr><th>流水号</th><th>类型</th><th>变动金额</th><th>变动后余额</th><th>关联订单</th><th>说明</th><th>时间</th></tr></thead><tbody>${state.ledgers.map(item => `<tr><td>${esc(item.id)}</td><td>${esc(item.type)}</td><td class="${item.amount >= 0 ? "positive" : "negative"}">${item.amount >= 0 ? "+" : ""}${money(item.amount)}</td><td>${money(item.balance_after)}</td><td>${esc(item.order_id || item.payment_order_id || "—")}</td><td>${esc(item.description)}</td><td>${time(item.created_at)}</td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无资金流水</td></tr>`}</tbody></table></div>${renderPager("ledgers")}</div>`;
}

function renderBalance() {
  const paymentBusy = state.busyAction === "payment";
  return pageHead("余额与充值", "充值用于申请邮箱；支付订单和资金流水分别记录，方便核对。", `<button class="ghost-btn" data-action="view" data-view="usage">查看资金流水</button>`) + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>创建充值订单</h2><strong>${money(state.user.balance)}</strong></div><div class="card-body form-grid">${state.paymentError ? `<div class="inline-error" role="alert">${icon("activity")}<span>${esc(state.paymentError)}</span></div>` : ""}<label>充值金额<input id="topup-amount" class="field" type="number" min="1" max="100000" step="0.01" value="50" ${paymentBusy ? "disabled" : ""}></label><label>支付方式<select id="topup-method" class="field" ${paymentBusy ? "disabled" : ""}>${state.paymentMethods.map(method => `<option value="${esc(method)}">${method === "alipay" ? "支付宝" : esc(method)}</option>`).join("")}</select></label><button class="primary-btn" data-action="create-payment" ${state.paymentMethods.length && !paymentBusy ? "" : "disabled"}>${paymentBusy ? "正在创建支付单…" : "前往支付"}</button>${state.paymentMethods.length ? "" : `<div class="notice warning">暂时没有可用支付方式，请联系管理员。</div>`}</div></div><div class="card balance-guide"><div class="card-head"><h2>余额怎么使用</h2></div><div class="card-body"><ol><li>选择目标平台并申请邮箱</li><li>系统预扣费用，收码成功后结算</li><li>超时未收到验证码自动退款</li></ol><button class="link-btn" data-action="view" data-view="apply">返回申请邮箱 →</button></div></div></div><div class="card"><div class="card-head"><h2>充值记录</h2></div><div class="table-wrap"><table><thead><tr><th>支付单</th><th>通道</th><th>金额</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.paymentOrders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.provider_name)}</td><td>${money(order.amount)}</td><td>${statusChip(order.status)}</td><td>${time(order.created_at)}</td><td>${order.status === "pending" && order.pay_url ? `<a class="link-btn" href="${esc(order.pay_url)}" target="_blank" rel="noopener">继续支付</a>` : "—"}</td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无充值记录</td></tr>`}</tbody></table></div>${renderPager("payments")}</div>`;
}

function renderAccountSettings(title, subtitle) {
  return pageHead(title, subtitle) + `<div class="account-grid"><div class="card"><div class="card-head"><h2>账户资料</h2>${icon("userCog", "section-icon")}</div><div class="card-body form-grid"><label>登录邮箱<input class="field" value="${esc(state.user.email)}" disabled></label><label>显示名称<input id="profile-name" class="field" value="${esc(state.user.display_name || "")}" maxlength="120" autocomplete="name"></label><button class="primary-btn" data-action="save-profile">保存账户资料</button></div></div><div class="card"><div class="card-head"><h2>登录密码</h2>${icon("shieldUser", "section-icon")}</div><div class="card-body form-grid"><label>当前密码<input id="current-password" class="field" type="password" maxlength="128" autocomplete="current-password"></label><label>新密码<input id="new-password" class="field" type="password" minlength="10" maxlength="128" autocomplete="new-password" placeholder="至少 10 位"></label><label>确认新密码<input id="confirm-password" class="field" type="password" minlength="10" maxlength="128" autocomplete="new-password"></label><button class="primary-btn" data-action="change-password">修改登录密码</button><div class="notice">修改后会撤销该账户的全部旧会话，当前浏览器将自动切换到新会话。</div></div></div></div><button class="danger-btn account-logout" data-action="logout">退出登录</button>`;
}

function renderSettings() {
  return renderAccountSettings("个人设置", "维护账户资料和登录密码。") + `<div class="user-shortcuts"><button class="ghost-btn" data-action="view" data-view="balance">余额与充值</button><button class="ghost-btn" data-action="view" data-view="keys">开发者工具</button></div>`;
}

function renderAdminAccount() {
  return renderAccountSettings("管理员账户", "修改管理员显示名称、登录密码和当前会话。");
}

function renderDocs() {
  const rows = [["GET", "/api/v1/services", "分页获取可申请平台、价格和余量"], ["GET", "/api/v1/services/{code}/availability", "查询单个平台实时余量"], ["POST", "/api/v1/orders", "创建注册订单并分配邮箱"], ["GET", "/api/v1/orders/{id}", "查询状态与验证码"], ["POST", "/api/v1/orders/{id}/submitted", "确认已提交平台注册"], ["POST", "/api/v1/orders/{id}/complete", "确认完成并结算"], ["POST", "/api/v1/orders/{id}/cancel", "取消任务并退款"], ["GET", "/api/v1/webhook-deliveries", "分页查询 Webhook 投递"]];
  return pageHead("API 文档", "用 Bearer API Key 管理注册收码任务。", "接口只返回完成任务所需的信息；邮箱密码、OAuth Token 和完整邮件始终留在服务端。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>方法</th><th>路径</th><th>用途</th></tr></thead><tbody>${rows.map(row => `<tr><td><code>${row[0]}</code></td><td><code>${row[1]}</code></td><td>${row[2]}</td></tr>`).join("")}</tbody></table></div><div class="card-body"><h3>鉴权</h3><pre class="code-sample">Authorization: Bearer hm_your_api_key</pre><h3>创建订单</h3><pre class="code-sample">POST /api/v1/orders\nContent-Type: application/json\n\n{\n  "service": "github",\n  "request_id": "client-request-001"\n}</pre><h3>查询验证码</h3><pre class="code-sample">GET /api/v1/orders/{id}</pre><p class="muted">当响应中的 <code>status</code> 为 <code>code_received</code> 时读取 <code>code</code>。列表响应包含 <code>data</code> 与 <code>pagination</code>；错误响应包含 <code>error</code> 与 <code>message</code>。</p></div></div>`;
}

function renderWebhooks() {
  return pageHead("Webhook", "订阅订单状态变化并查看投递结果。") + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>端点</h2></div><div class="table-wrap"><table><thead><tr><th>URL</th><th>事件</th><th>状态</th><th>操作</th></tr></thead><tbody>${state.webhookEndpoints.map(endpoint => `<tr><td>${esc(endpoint.url)}</td><td>${esc(endpoint.events.join(", "))}</td><td>${endpoint.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><button class="link-btn" data-action="delete-webhook" data-id="${endpoint.id}">删除</button></td></tr>`).join("") || `<tr><td colspan="4" class="empty">暂无 Webhook 端点</td></tr>`}</tbody></table></div>${renderPager("webhooks")}</div><div class="card"><div class="card-head"><h2>新增端点</h2></div><div class="card-body form-grid"><label>接收地址<input id="webhook-url" class="field" type="url" placeholder="https://example.com/webhook"></label><button class="primary-btn" data-action="create-webhook">创建端点</button><div class="notice">签名密钥只显示一次。请求使用 <code>X-HeroMail-Signature</code> 验证。</div></div></div></div><div class="card" style="margin-top:16px"><div class="card-head"><h2>投递记录</h2></div><div class="table-wrap"><table><thead><tr><th>事件</th><th>订单</th><th>状态</th><th>尝试</th><th>响应</th><th>下次重试</th><th>操作</th></tr></thead><tbody>${state.webhookDeliveries.map(item => `<tr><td>${esc(item.event)}</td><td>${esc(item.order_id)}</td><td>${statusChip(item.status)}</td><td>${item.attempts}</td><td>${item.response_code || "—"}</td><td>${time(item.next_retry_at)}</td><td>${item.status === "failed" ? `<button class="link-btn" data-action="retry-webhook" data-id="${item.id}">重试</button>` : "—"}</td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无投递记录</td></tr>`}</tbody></table></div>${renderPager("webhook-deliveries")}</div>`;
}

async function importMailboxes() {
  const file = document.querySelector("#mailbox-file")?.files?.[0];
  if (!file) return toast("请选择 TXT 或 CSV 文件");
  const form = new FormData(); form.append("file", file);
  const result = await api("/api/v1/admin/mailboxes/import", { method: "POST", body: form });
  await refresh();
  toast(`导入完成：${result.data.imported} 个成功，${result.data.failed} 个失败`);
}

function renderAdminMailboxes() {
  const total = state.pagination.mailboxes?.total || 0;
  const rows = state.mailboxes.map(mailbox => `<tr><td>${esc(mailbox.address)}</td><td>${esc(providerLabel(mailbox.provider))}</td><td>${esc(connectionLabel(mailbox.connection_method))}</td><td>${statusChip(mailbox.verification_status || mailbox.state)}${mailbox.verification_error ? `<div class="muted">${esc(mailbox.verification_error)}</div>` : ""}</td><td>${esc((mailbox.registered_platforms || []).join(", ") || "—")}</td><td>${mailbox.health_score}/100</td><td>${time(mailbox.last_verified_at)}</td><td><div class="table-actions"><button class="link-btn" data-action="mailbox-messages" data-id="${esc(mailbox.id)}" data-address="${esc(mailbox.address)}">收件</button><button class="link-btn" data-action="verify-mailbox" data-id="${esc(mailbox.id)}">验证</button><button class="link-btn danger-text" data-action="delete-mailbox" data-id="${esc(mailbox.id)}">删除</button></div></td></tr>`).join("");
  const outlookCount = state.overview?.outlook_mailboxes ?? 0;
  const outlookDECount = state.overview?.outlook_de_mailboxes ?? 0;
  const hotmailCount = state.overview?.hotmail_mailboxes ?? 0;
  const pendingCount = state.overview?.pending_mailboxes ?? 0;
  const verifiedCount = state.overview?.verified_mailboxes ?? 0;
  return pageHead("邮箱池", "一个统一资源池，按 Outlook、Outlook.de 和 Hotmail 分类并自动验证。", `<button class="ghost-btn" data-action="refresh">刷新状态</button>`) + `<section class="mailbox-import-hero"><div class="mailbox-import-copy"><span class="eyebrow">MAILBOX POOL</span><h2>导入邮箱账号</h2><p>把账号文件拖进来即可。系统按行读取，不占满内存，自动识别邮箱类型并优先使用 Graph 验证，失败时回退 IMAP。</p><div class="import-capabilities"><span><i class="cap-dot blue"></i>Outlook</span><span><i class="cap-dot green"></i>Outlook.de</span><span><i class="cap-dot purple"></i>Hotmail</span><span><i class="cap-dot green"></i>自动验证</span></div></div><div class="mailbox-dropzone" data-action="pick-mailbox-file"><input id="mailbox-file" type="file" accept=".txt,.csv,.jsonl,text/plain,text/csv,application/json"><div class="drop-icon">↥</div><strong>拖放 TXT / CSV / JSON Lines</strong><span>或点击选择文件 · 支持大文件逐行导入</span><em id="mailbox-file-name">尚未选择文件</em></div></section><div class="mailbox-import-actions"><button class="primary-btn" data-action="import-mailboxes">导入并开始验证</button><span>系统唯一邮箱池 · 不需要创建或选择池</span></div><div class="mailbox-metrics"><div><span>全部邮箱</span><strong>${total}</strong><small>统一资源池</small></div><div><span>Outlook</span><strong>${outlookCount}</strong><small>国际域名</small></div><div><span>Outlook.de</span><strong>${outlookDECount}</strong><small>德国域名</small></div><div><span>Hotmail</span><strong>${hotmailCount}</strong><small>独立分类</small></div><div><span>待验证</span><strong>${pendingCount}</strong><small>后台队列</small></div><div><span>已验证</span><strong>${verifiedCount}</strong><small>可参与分配</small></div></div><div class="card mailbox-table-card"><div class="card-head"><div><h2>邮箱明细</h2><span class="muted">服务端分页 · 已注册平台默认为空，收码结算后自动标记</span></div><span class="pool-singleton">唯一邮箱池</span></div><div class="table-wrap"><table><thead><tr><th>邮箱</th><th>类型</th><th>连接方式</th><th>验证状态</th><th>已注册平台</th><th>健康分</th><th>最近验证</th><th>操作</th></tr></thead><tbody>${rows || `<tr><td colspan="8" class="empty">还没有邮箱，导入文件后会显示在这里</td></tr>`}</tbody></table></div>${renderPager("mailboxes")}</div>`;
}

function renderAdminUsers() {
  const tab = state.adminTabs.users;
  const tabs = sectionTabs("users", tab, [["accounts", "用户账户", state.pagination.users?.total], ["ledger", "资金流水", state.pagination["admin-ledger"]?.total]]);
  if (tab === "ledger") {
    const rows = state.ledgers.map(item => `<tr><td data-label="用户 ID">${esc(item.user_id || "—")}</td><td data-label="流水号"><code>${esc(item.id)}</code></td><td data-label="类型">${esc(item.type)}</td><td data-label="金额" class="${item.amount >= 0 ? "positive" : "negative"}">${item.amount >= 0 ? "+" : ""}${money(item.amount)}</td><td data-label="变动后余额">${money(item.balance_after)}</td><td data-label="关联业务">${esc(item.order_id || item.payment_order_id || "—")}</td><td data-label="原因">${esc(item.description)}</td><td data-label="时间">${time(item.created_at)}</td></tr>`).join("");
    return pageHead("用户与余额", "在同一处管理用户账户和每一笔资金变动。") + tabs + `${state.ledgerUserID ? `<div class="active-filter"><span>正在查看用户 <code>${esc(state.ledgerUserID)}</code> 的流水</span><button class="link-btn" data-action="clear-ledger-user">查看全部流水</button></div>` : ""}<div class="card"><div class="table-wrap responsive-admin-table"><table><thead><tr><th>用户 ID</th><th>流水号</th><th>类型</th><th>金额</th><th>变动后余额</th><th>关联业务</th><th>原因</th><th>时间</th></tr></thead><tbody>${rows || `<tr><td colspan="8" class="empty">暂无资金流水</td></tr>`}</tbody></table></div>${renderPager("admin-ledger")}</div>`;
  }
  const rows = state.users.map(user => `<tr><td data-label="用户"><strong>${esc(user.email)}</strong><span class="table-subline">${esc(user.id)}</span></td><td data-label="显示名称">${esc(user.display_name || "—")}</td><td data-label="角色">${user.role === "admin" ? "管理员" : "普通用户"}</td><td data-label="状态">${statusChip(user.status)}</td><td data-label="余额"><strong>${money(user.balance)}</strong></td><td data-label="创建时间">${time(user.created_at)}</td><td data-label="操作"><div class="table-actions"><button class="link-btn" data-action="view-user-ledger" data-id="${esc(user.id)}">查看流水</button><button class="link-btn" data-action="adjust-balance" data-id="${esc(user.id)}">调整余额</button></div></td></tr>`).join("");
  return pageHead("用户与余额", "账户资料、可用余额和人工补偿集中管理。") + tabs + `<div class="card"><div class="table-wrap responsive-admin-table"><table><thead><tr><th>用户</th><th>显示名称</th><th>角色</th><th>状态</th><th>余额</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${rows || `<tr><td colspan="7" class="empty">暂无用户</td></tr>`}</tbody></table></div>${renderPager("users")}</div>`;
}

function renderAdminPayments() {
  const tab = state.adminTabs.payments;
  const providerRows = state.paymentProviders.map(item => `<tr><td><strong>${esc(item.name)}</strong></td><td>${item.type === "easypay" ? "易支付" : "支付宝官方"}</td><td>${esc(item.methods.map(method => method === "alipay" ? "支付宝" : method).join(", "))}</td><td>${item.priority}</td><td>${item.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><div class="table-actions"><button class="link-btn" data-action="edit-payment-provider" data-id="${esc(item.id)}">编辑</button><button class="link-btn danger-text" data-action="delete-payment-provider" data-id="${esc(item.id)}" data-name="${esc(item.name)}">删除</button></div></td></tr>`).join("");
  const orderRows = state.paymentOrders.map(item => `<tr><td data-label="支付单">${esc(item.id)}</td><td data-label="用户">${esc(item.user_id)}</td><td data-label="服务商">${esc(item.provider_name)}</td><td data-label="金额">${money(item.amount)}</td><td data-label="状态">${statusChip(item.status)}</td><td data-label="上游流水">${esc(item.provider_trade_no || "—")}</td><td data-label="创建时间">${time(item.created_at)}</td></tr>`).join("");
  const tabs = sectionTabs("payments", tab, [["orders", "充值订单", state.pagination["admin-payments"]?.total], ["providers", "支付通道", state.pagination["payment-providers"]?.total]]);
  if (tab === "providers") return pageHead("支付管理", "支付通道和充值订单分开维护，减少误操作。", `<button class="primary-btn" data-action="edit-payment-provider">新增支付通道</button>`) + tabs + `<div class="provider-guidance"><div><span class="provider-badge easypay">易</span><strong>易支付</strong><p>API 地址、PID 和 PKey。</p></div><div><span class="provider-badge alipay">支</span><strong>支付宝官方</strong><p>AppID、应用私钥和支付宝公钥。</p></div></div><div class="card"><div class="card-head"><h2>支付通道</h2><span class="muted">敏感凭证使用 AES-256-GCM 加密保存</span></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>类型</th><th>支付方式</th><th>优先级</th><th>状态</th><th>操作</th></tr></thead><tbody>${providerRows || `<tr><td colspan="6" class="empty">暂无支付通道，用户暂时无法充值</td></tr>`}</tbody></table></div>${renderPager("payment-providers")}</div>`;
  return pageHead("支付管理", "先处理用户充值订单，需要时再切换到支付通道。") + tabs + `<div class="card"><div class="table-wrap responsive-admin-table"><table><thead><tr><th>支付单</th><th>用户</th><th>服务商</th><th>金额</th><th>状态</th><th>上游流水</th><th>创建时间</th></tr></thead><tbody>${orderRows || `<tr><td colspan="7" class="empty">暂无充值订单</td></tr>`}</tbody></table></div>${renderPager("admin-payments")}</div>`;
}

async function showPaymentProviderEditor(providerID = "") {
  const provider = state.paymentProviders.find(item => item.id === providerID) || { id: "", name: "", type: "easypay", enabled: true, priority: 100 };
  const editing = Boolean(provider.id);
  const details = editing ? (await api(`/api/v1/admin/payment/providers/${provider.id}`)).data : { config: {}, configured: {} };
  const providerConfig = details.config || {};
  const configured = details.configured || {};
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal payment-modal"><div class="card-head"><div><h2>${editing ? "编辑" : "新增"}支付服务商</h2><div class="modal-subtitle">选择服务商类型后填写对应凭证</div></div><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body form-grid"><input id="pay-provider-id" type="hidden" value="${esc(provider.id)}"><div class="form-columns"><label>服务商名称<input id="pay-provider-name" class="field" value="${esc(provider.name)}" placeholder="主支付通道"></label><label>服务商类型<select id="pay-provider-type" class="field" ${editing ? "disabled" : ""}><option value="easypay" ${provider.type === "easypay" ? "selected" : ""}>易支付</option><option value="alipay" ${provider.type === "alipay" ? "selected" : ""}>支付宝官方</option></select></label></div><div id="easypay-fields" class="provider-fields"><div class="provider-form-title"><span class="provider-badge easypay">易</span><div><strong>易支付配置</strong><span>兼容易支付协议的聚合支付服务商</span></div></div><label>API 地址<input id="pay-easypay-api-base" class="field" type="url" value="${esc(providerConfig.api_base || "")}" placeholder="https://pay.example.com/submit.php"></label><div class="form-columns"><label>商户 ID（PID）<input id="pay-easypay-pid" class="field" value="${esc(providerConfig.pid || "")}" autocomplete="off"></label><label>支付宝通道 ID（可选）<input id="pay-easypay-channel" class="field" value="${esc(providerConfig.channel_id || "")}" autocomplete="off"></label></div><label>商户密钥（PKey）<input id="pay-easypay-pkey" class="field" type="password" autocomplete="new-password" placeholder="${configured.pkey ? "已配置，留空则保留" : "请输入商户密钥"}"></label></div><div id="alipay-fields" class="provider-fields"><div class="provider-form-title"><span class="provider-badge alipay">支</span><div><strong>支付宝官方配置</strong><span>直接接入支付宝开放平台 RSA2</span></div></div><label>API 网关<input class="field" value="https://openapi.alipay.com/gateway.do" disabled></label><label>应用 AppID<input id="pay-alipay-app-id" class="field" value="${esc(providerConfig.app_id || "")}" autocomplete="off"></label><label>应用私钥<textarea id="pay-alipay-private-key" class="field" rows="5" autocomplete="off" placeholder="${configured.private_key ? "已配置，留空则保留" : "-----BEGIN PRIVATE KEY-----"}"></textarea></label><label>支付宝公钥<textarea id="pay-alipay-public-key" class="field" rows="5" autocomplete="off" placeholder="${configured.public_key ? "已配置，留空则保留" : "-----BEGIN PUBLIC KEY-----"}"></textarea></label></div><div class="form-columns"><label>优先级<input id="pay-provider-priority" class="field" type="number" min="1" value="${Number(provider.priority || 100)}"></label><label class="check-label provider-enabled"><input id="pay-provider-enabled" type="checkbox" ${provider.enabled ? "checked" : ""}> 启用该服务商</label></div><button class="primary-btn" data-action="save-payment-provider">${editing ? "保存修改" : "创建服务商"}</button></div></div></div>`);
  const typeSelect = document.querySelector("#pay-provider-type");
  const toggleFields = () => {
    const type = typeSelect.value;
    document.querySelector("#easypay-fields").hidden = type !== "easypay";
    document.querySelector("#alipay-fields").hidden = type !== "alipay";
  };
  typeSelect.addEventListener("change", toggleFields);
  toggleFields();
}

function renderAdminSettings() {
  const version = state.version || { current_version: "unknown", commit: "unknown", online_upgrade_enabled: false, upgrade: { state: "idle", message: "" } };
  const release = version.latest_release || {}; const hasRelease = Boolean(release.tag);
  const normalizeReleaseVersion = value => String(value || "").trim().replace(/^v/i, "");
  const upgradeAvailable = hasRelease && normalizeReleaseVersion(version.current_version) !== normalizeReleaseVersion(release.tag);
  const running = ["backing_up", "queued", "updating"].includes(version.upgrade.state);
  const upgradeLabel = running ? "升级进行中" : upgradeAvailable ? `升级到 ${esc(release.tag)}` : hasRelease ? "当前已是最新版本" : "暂未获取正式版本";
  const stageIndex = ({ idle: -1, backing_up: 0, queued: 1, updating: 2, success: 4, failed: -1 })[version.upgrade.state] ?? -1;
  const stages = ["备份数据库", "提交任务", "拉取并重启", "健康检查"];
  const stageMarkup = stages.map((label, index) => `<div class="upgrade-step ${stageIndex > index ? "done" : stageIndex === index ? "active" : ""}"><span>${stageIndex > index ? "✓" : index + 1}</span><strong>${label}</strong></div>`).join("");
  const unavailableReason = !version.online_upgrade_enabled ? "当前部署未配置在线升级器或数据库备份目录。" : !hasRelease ? "暂时无法连接 GitHub Release，请稍后重新检查。" : "";
  return pageHead("版本升级", "只负责检查正式版本、备份数据库和切换官方镜像。", `<button class="ghost-btn" data-action="check-updates" ${running ? "disabled" : ""}>重新检查</button>`) + `<section class="version-console"><div class="version-route"><div><span>当前运行</span><strong>${esc(version.current_version)}</strong><small><code>${esc(String(version.commit || "").slice(0, 12))}</code></small></div><span class="version-arrow">→</span><div><span>GitHub 正式版</span><strong>${esc(release.tag || "未获取")}</strong><small>${release.published_at ? time(release.published_at) : "等待检查"}</small></div></div><div class="version-action"><div class="detail-status">${statusChip(version.upgrade.state)}<span>${esc(version.upgrade.message || "尚未执行在线升级")}</span></div><button class="primary-btn" data-action="upgrade" ${version.online_upgrade_enabled && upgradeAvailable && !running ? "" : "disabled"}>${upgradeLabel}</button>${unavailableReason ? `<p class="inline-error">${esc(unavailableReason)}</p>` : ""}</div></section><div class="upgrade-progress" aria-label="升级流程">${stageMarkup}</div><div class="upgrade-safety"><div>${icon("database")}<span><strong>自动备份</strong>升级前创建并校验 PostgreSQL 备份</span></div><div>${icon("shieldUser")}<span><strong>数据卷不变</strong>PostgreSQL 与 Redis 数据卷不会替换</span></div><div>${icon("activity")}<span><strong>健康确认</strong>新容器通过检查后才标记成功</span></div></div><details class="release-details"><summary><span>版本日志 · ${esc(release.tag || "暂无")}</span><span>展开查看</span></summary><div>${hasRelease ? `<pre class="release-notes">${esc(release.notes || "该版本未提供更新日志。")}</pre>${release.url ? `<a class="link-btn release-link" href="${esc(release.url)}" target="_blank" rel="noopener">在 GitHub 查看完整 Release</a>` : ""}` : `<div class="empty">暂时没有可显示的正式版本日志</div>`}</div></details>`;
}

function renderAdminChannels() { return pageHead("接入渠道", "通过 Microsoft OAuth2 接入统一邮箱池。") + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>Microsoft Graph OAuth2</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> OAuth 授权流程 <span class="status-ok">服务端验证</span></div><div class="status-item"><i class="status-dot"></i> Token 存储 <span class="status-ok">AES-256-GCM</span></div><div class="status-item"><i class="status-dot"></i> 已接入邮箱 <strong style="margin-left:auto">${state.pagination.mailboxes?.total || 0}</strong></div></div></div></div><div class="card"><div class="card-head"><h2>连接 Microsoft 邮箱</h2></div><div class="card-body form-grid"><button class="primary-btn" data-action="microsoft-oauth">连接 Microsoft 邮箱</button><div class="notice">OAuth 完成后自动识别 Outlook、Outlook.de 或 Hotmail，并加入唯一邮箱池。</div></div></div></div>`; }

function renderAdminOperations() {
  const tab = state.adminTabs.operations;
  const tabs = sectionTabs("operations", tab, [["alerts", "运行异常"], ["audit", "审计记录", state.pagination.audit?.total]]);
  if (tab === "audit") return pageHead("运维中心", "运行异常用于处置，审计记录用于追溯。") + tabs + `<div class="card"><div class="table-wrap responsive-admin-table"><table><thead><tr><th>操作者</th><th>动作</th><th>资源</th><th>详情</th><th>IP</th><th>时间</th></tr></thead><tbody>${state.auditLogs.map(item => `<tr><td data-label="操作者">${esc(item.actor_id)}</td><td data-label="动作"><code>${esc(item.action)}</code></td><td data-label="资源">${esc(item.resource_type)} · ${esc(item.resource_id)}</td><td data-label="详情" class="wrap-cell">${esc(item.detail)}</td><td data-label="IP">${esc(item.ip || "—")}</td><td data-label="时间">${time(item.created_at)}</td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无审计记录</td></tr>`}</tbody></table></div>${renderPager("audit")}</div>`;
  const overview = state.overview || {};
  const alerts = [];
  if (!overview.available_mailboxes) alerts.push(["库存不可用", "当前没有可分配邮箱，用户无法创建新订单。", "red", "admin-mailboxes", "检查邮箱池"]);
  if (overview.auth_errors) alerts.push(["邮箱授权异常", `${overview.auth_errors} 个邮箱需要重新授权或重新验证。`, "orange", "admin-mailboxes", "处理邮箱"]);
  if (overview.blocked_mailboxes) alerts.push(["邮箱已隔离", `${overview.blocked_mailboxes} 个邮箱已被系统隔离。`, "red", "admin-mailboxes", "查看隔离项"]);
  return pageHead("运维中心", "只展示当前需要处理的运行问题，历史操作在审计记录中查看。", `<button class="ghost-btn" data-action="refresh">刷新</button>`) + tabs + `<div class="ops-list">${alerts.map(item => `<article class="ops-alert"><i class="status-dot ${item[2]}"></i><div><strong>${item[0]}</strong><p>${item[1]}</p></div><button class="ghost-btn" data-action="view" data-view="${item[3]}">${item[4]}</button></article>`).join("") || `<div class="healthy-state">${icon("activity")}<strong>当前运行正常</strong><span>邮箱库存、授权和隔离状态均无待处理问题。</span></div>`}</div>`;
}


function render() {
  if (!state.user) { redirectToLogin(); return Promise.resolve(); }
  document.body.classList.toggle("admin-shell", state.role === "admin");
  document.body.classList.toggle("user-shell", state.role !== "admin");
  document.body.classList.remove("auth-mode");
  renderNav();
  const views = { apply: renderApply, current: renderCurrent, orders: () => renderOrders() + renderPager("orders"), docs: renderDocs, keys: renderAPIKeys, webhooks: renderWebhooks, usage: renderUsage, balance: renderBalance, settings: renderSettings, "admin-overview": renderAdminOverview, "admin-mailboxes": renderAdminMailboxes, "admin-channels": renderAdminChannels, "admin-services": renderAdminServices, "admin-orders": renderAdminOrders, "admin-users": renderAdminUsers, "admin-payments": renderAdminPayments, "admin-operations": renderAdminOperations, "admin-settings": renderAdminSettings, "admin-account": renderAdminAccount };
  const content = state.loading ? `<div class="portal-loading" role="status" aria-live="polite">${icon("activity")}<strong>正在加载${state.role === "admin" ? "运营数据" : "工作台"}…</strong><span>数据更新后会自动显示</span></div>` : (views[state.view] || views.apply)();
  const error = state.pageError ? `<div class="page-error" role="alert"><div>${icon("activity")}<span><strong>当前页面加载失败</strong>${esc(state.pageError)}</span></div><button class="ghost-btn" data-action="refresh">重新加载</button></div>` : "";
  document.querySelector("#content").innerHTML = error + content;
  setPageUpdating(false);
  updateAccountChrome();
  return Promise.resolve();
}

function updateAccountChrome() {
  document.querySelector("#balance").textContent = `余额 ${money(state.user.balance)}`;
  document.querySelector(".avatar").textContent = (state.user.display_name || state.user.email || "U").slice(0, 1).toUpperCase();
  if (state.version?.current_version) document.querySelector("#app-version").textContent = state.version.current_version;
}

function setPageUpdating(updating) {
  const content = document.querySelector("#content");
  if (!content) return;
  content.classList.toggle("is-updating", updating);
  if (updating) content.setAttribute("aria-busy", "true"); else content.removeAttribute("aria-busy");
}

function rememberPage(key, body) { if (body.pagination) state.pagination[key] = body.pagination; return body.data || []; }
function requestedPage(key) { return state.pagination[key]?.page || 1; }
async function loadUser() {
  const filteringOrders = state.view === "orders";
  const page = filteringOrders ? state.pagination.orders?.page || 1 : 1;
  const params = new URLSearchParams({ page: String(page), page_size: "20" });
  if (filteringOrders && state.userOrderFilters.status) params.set("status", state.userOrderFilters.status);
  if (filteringOrders && state.userOrderFilters.service) params.set("service", state.userOrderFilters.service);
  if (filteringOrders && state.userOrderFilters.query) params.set("query", state.userOrderFilters.query);
  const [me, services, orders] = await Promise.all([api("/api/v1/me"), api("/api/v1/services?page=1&page_size=100"), api(`/api/v1/orders?${params}`)]);
  state.user = me; state.services = services.data || []; state.orders = rememberPage("orders", orders); if (!state.services.some(service => service.code === state.selectedService) && state.services[0]) state.selectedService = state.services[0].code;
  if (!state.currentOrder || !["assigned", "waiting_code", "code_received"].includes(state.currentOrder.status)) state.currentOrder = state.orders.find(order => ["assigned", "waiting_code", "code_received"].includes(order.status)) || null;
  if (["keys", "usage", "balance", "webhooks"].includes(state.view)) await loadUserModule(state.view);
  if (["apply", "current"].includes(state.view) && state.currentOrder && !state.polling && ["assigned", "waiting_code", "code_received"].includes(state.currentOrder.status)) startPolling(state.currentOrder.id);
}
async function loadUserModule(view) {
  if (view === "keys") state.apiKeys = rememberPage("keys", await api(`/api/v1/api-keys?page=${requestedPage("keys")}&page_size=20`));
  if (view === "usage") state.ledgers = rememberPage("ledgers", await api(`/api/v1/wallet/ledgers?page=${requestedPage("ledgers")}&page_size=20`));
  if (view === "balance") { state.paymentMethods = (await api("/api/v1/payment/methods")).data || []; state.paymentOrders = rememberPage("payments", await api(`/api/v1/payment/orders?page=${requestedPage("payments")}&page_size=20`)); }
  if (view === "webhooks") { state.webhookEndpoints = rememberPage("webhooks", await api(`/api/v1/webhooks?page=${requestedPage("webhooks")}&page_size=20`)); state.webhookDeliveries = rememberPage("webhook-deliveries", await api(`/api/v1/webhook-deliveries?page=${requestedPage("webhook-deliveries")}&page_size=20`)); }
}
async function loadAdmin() {
  state.user = await api("/api/v1/me");
  if (["admin-overview", "admin-mailboxes"].includes(state.view) || (state.view === "admin-operations" && state.adminTabs.operations === "alerts")) state.overview = (await api("/api/v1/admin/overview")).data;
  if (["admin-overview", "admin-services", "admin-orders"].includes(state.view)) state.services = rememberPage("admin-services", await api(`/api/v1/admin/services?page=${requestedPage("admin-services")}&page_size=20`));
  if (state.view === "admin-mailboxes") state.mailboxes = rememberPage("mailboxes", await api(`/api/v1/admin/mailboxes?page=${requestedPage("mailboxes")}&page_size=20`));
  if (state.view === "admin-overview") state.orders = rememberPage("admin-orders", await api("/api/v1/admin/orders?page=1&page_size=6"));
  if (state.view === "admin-orders") {
    const params = new URLSearchParams({ page: requestedPage("admin-orders"), page_size: 20 });
    if (state.orderFilters.status) params.set("status", state.orderFilters.status);
    if (state.orderFilters.service) params.set("service", state.orderFilters.service);
    if (state.orderFilters.query) params.set("query", state.orderFilters.query);
    const selectedID = state.currentOrder?.id;
    state.orders = rememberPage("admin-orders", await api(`/api/v1/admin/orders?${params}`));
    state.currentOrder = state.orders.find(order => order.id === selectedID) || null;
  }
  if (state.view === "admin-users" && state.adminTabs.users === "accounts") state.users = rememberPage("users", await api(`/api/v1/admin/users?page=${requestedPage("users")}&page_size=20`));
  if (state.view === "admin-users" && state.adminTabs.users === "ledger") {
    const userFilter = state.ledgerUserID ? `&user_id=${encodeURIComponent(state.ledgerUserID)}` : "";
    state.ledgers = rememberPage("admin-ledger", await api(`/api/v1/admin/wallet/ledgers?page=${requestedPage("admin-ledger")}&page_size=20${userFilter}`));
  }
  if (state.view === "admin-operations" && state.adminTabs.operations === "audit") state.auditLogs = rememberPage("audit", await api(`/api/v1/admin/audit-logs?page=${requestedPage("audit")}&page_size=20`));
  if (state.view === "admin-payments" && state.adminTabs.payments === "providers") state.paymentProviders = rememberPage("payment-providers", await api(`/api/v1/admin/payment/providers?page=${requestedPage("payment-providers")}&page_size=20`));
  if (state.view === "admin-payments" && state.adminTabs.payments === "orders") state.paymentOrders = rememberPage("admin-payments", await api(`/api/v1/admin/payment/orders?page=${requestedPage("admin-payments")}&page_size=20`));
  if (state.view === "admin-settings") state.version = (await api("/api/v1/admin/system/version")).data;
}

async function refresh() {
  state.pageError = "";
  const content = document.querySelector("#content");
  const initialLoad = !content?.firstElementChild || Boolean(content.querySelector(".portal-loading"));
  state.loading = initialLoad;
  if (initialLoad) await render();
  else {
    renderNav();
    setPageUpdating(true);
  }
  try {
    if (!state.user && !state.token) { redirectToLogin(); return; }
    if (state.role === "admin") await loadAdmin(); else await loadUser();
  } catch (error) {
    state.pageError = error.message;
    toast(error.message);
  } finally {
    state.loading = false;
  }
  await render();
}
function stopPolling() { if (state.polling) { clearInterval(state.polling); state.polling = null; } }
function startPolling(orderID) {
  stopPolling();
  state.polling = setInterval(async () => {
    try {
      const previous = state.currentOrder;
      const result = await api(`/api/v1/orders/${orderID}`);
      state.currentOrder = result.data;
      const index = state.orders.findIndex(order => order.id === orderID);
      if (index >= 0) state.orders[index] = result.data;
      const taskChanged = previous?.status !== result.data.status || previous?.code !== result.data.code || previous?.mailbox_address !== result.data.mailbox_address;
      if (state.view === "apply") {
        if (taskChanged) await updateApplyView({ task: true, recent: true }); else updateTaskCountdown(result.data);
      } else if (state.view === "current") {
        if (taskChanged) await updateCurrentTaskView(); else updateTaskCountdown(result.data);
      }
      if (["completed", "canceled", "expired_refunded", "allocation_failed", "disputed"].includes(result.data.status)) stopPolling();
    } catch (error) { stopPolling(); }
  }, 1000);
}
async function createOrder() {
  if (state.busy) return;
  const service = selectedService();
  if (Number(state.user?.balance || 0) < Number(service.price || 0)) { state.orderError = "余额不足，请先充值后再申请。"; await updateApplyView({ summary: true }); return; }
  state.orderError = ""; state.busy = true; await updateApplyView({ summary: true });
  try {
    const result = await api("/api/v1/orders", { method: "POST", body: JSON.stringify({ service: service.code, request_id: `web-${Date.now()}` }) });
    state.currentOrder = result.data;
    await loadUser();
    toast("邮箱已分配，请完成平台注册");
  } catch (error) {
    state.orderError = /insufficient balance|余额|balance/i.test(error.message) ? "余额不足，请先充值。" : error.message;
    toast(error.message);
  } finally {
    state.busy = false;
    updateAccountChrome();
    await updateApplyView({ all: true });
  }
}
async function mutateOrder(action) {
  if (!state.currentOrder || state.busyAction) return;
  if (action === "cancel" && !window.confirm("确认取消任务并退款吗？")) return;
  state.busyAction = action;
  if (state.view === "apply") await updateApplyView({ task: true }); else await updateCurrentTaskView();
  try {
    const result = await api(`/api/v1/orders/${state.currentOrder.id}/${action}`, { method: "POST" });
    state.currentOrder = result.data;
    await loadUser();
    if (action === "submitted") startPolling(result.data.id); else if (["complete", "cancel"].includes(action)) stopPolling();
    toast(action === "submitted" ? "已进入收码等待" : "订单状态已更新");
  } catch (error) { toast(error.message); }
  finally {
    state.busyAction = "";
    updateAccountChrome();
    if (state.view === "apply") await updateApplyView({ all: true }); else await updateCurrentTaskView();
  }
}
async function selectOrder(id) { const found = state.orders.find(order => order.id === id); if (found) { state.currentOrder = found; state.view = state.role === "admin" ? "admin-orders" : "orders"; await render(); } }

function showSecret(title, secret) {
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal"><div class="card-head"><h2>${esc(title)}</h2><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body"><div class="notice warning">此密钥只显示一次。</div><div class="secret-value"><code>${esc(secret)}</code><button class="ghost-btn" data-action="copy" data-copy="${esc(secret)}">复制</button></div></div></div></div>`);
}

function showServiceEditor(serviceID = "") {
  const service = state.services.find(item => item.id === serviceID) || { id: "", code: "", name: "", description: "", enabled: true, allowed_providers: ["outlook", "outlook_de", "hotmail"], price: 0.35, ttl_seconds: 600, sender_domains: [], subject_keywords: [], regex: "\\b(\\d{6})\\b" };
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal service-modal"><div class="card-head"><div><h2>${service.id ? "编辑" : "新建"}目标平台</h2><div class="modal-subtitle">邮箱分配和验证码匹配都在这里配置</div></div><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body form-grid service-modal-body"><input id="service-id" type="hidden" value="${esc(service.id)}"><section class="service-config-section"><h3>基本信息</h3><div class="form-columns"><label>平台代码<input id="service-code" class="field" value="${esc(service.code)}" placeholder="github"></label><label>平台名称<input id="service-name" class="field" value="${esc(service.name)}" placeholder="GitHub"></label></div><label>说明<input id="service-description" class="field" value="${esc(service.description)}" placeholder="注册验证码"></label><div class="form-columns"><label>单价<input id="service-price" class="field" type="number" min="0" step="0.01" value="${Number(service.price)}"></label><label>任务有效期（秒）<input id="service-ttl" class="field" type="number" min="60" max="86400" value="${Number(service.ttl_seconds)}"></label></div></section><section class="service-config-section"><h3>可用邮箱</h3><fieldset class="provider-options"><legend>订单只会从选中的邮箱类型中分配</legend><label><input type="checkbox" name="service-provider" value="outlook" ${(service.allowed_providers || []).includes("outlook") ? "checked" : ""}> Outlook</label><label><input type="checkbox" name="service-provider" value="outlook_de" ${(service.allowed_providers || []).includes("outlook_de") ? "checked" : ""}> Outlook.de</label><label><input type="checkbox" name="service-provider" value="hotmail" ${(service.allowed_providers || []).includes("hotmail") ? "checked" : ""}> Hotmail</label></fieldset></section><section class="service-config-section"><h3>邮件匹配</h3><label>发件人域名<input id="service-senders" class="field" value="${esc((service.sender_domains || []).join(", "))}" placeholder="github.com"></label><label>主题关键词<input id="service-subjects" class="field" value="${esc((service.subject_keywords || []).join(", "))}" placeholder="verification, 验证码"></label><label>验证码正则<input id="service-regex" class="field utility-field" value="${esc(service.regex)}"></label></section></div><div class="service-modal-footer"><label class="check-label"><input id="service-enabled" type="checkbox" ${service.enabled ? "checked" : ""}> 启用该目标平台</label><button class="primary-btn" data-action="save-service">保存目标平台</button></div></div></div>`);
}

const splitList = value => String(value || "").split(",").map(item => item.trim().toLowerCase()).filter(Boolean);
async function saveService() {
  const allowedProviders = [...document.querySelectorAll('input[name="service-provider"]:checked')].map(input => input.value);
  const payload = { id: document.querySelector("#service-id")?.value, code: document.querySelector("#service-code")?.value.trim().toLowerCase(), name: document.querySelector("#service-name")?.value.trim(), description: document.querySelector("#service-description")?.value.trim(), enabled: document.querySelector("#service-enabled")?.checked, allowed_providers: allowedProviders, price: Number(document.querySelector("#service-price")?.value), ttl_seconds: Number(document.querySelector("#service-ttl")?.value), sender_domains: splitList(document.querySelector("#service-senders")?.value), subject_keywords: splitList(document.querySelector("#service-subjects")?.value), regex: document.querySelector("#service-regex")?.value.trim() };
  if (!payload.code || !payload.name || !payload.regex || !payload.allowed_providers.length) return toast("请完整填写平台配置");
  if (payload.allowed_providers.some(provider => !["outlook", "outlook_de", "hotmail"].includes(provider))) return toast("邮箱类型配置无效");
  if (!payload.sender_domains.length) return toast("至少填写一个发件人域名");
  if (!Number.isFinite(payload.price) || payload.price < 0) return toast("单价不能小于 0");
  if (!Number.isInteger(payload.ttl_seconds) || payload.ttl_seconds < 60 || payload.ttl_seconds > 86400) return toast("有效期必须在 60 到 86400 秒之间");
  await api("/api/v1/admin/services", { method: "POST", body: JSON.stringify(payload) });
  document.querySelector("#secret-modal")?.remove(); await refresh(); toast("目标平台已保存");
}

async function deleteService(id) { if (!window.confirm("确定删除该目标平台？已有订单的平台不能删除，可改为停用。")) return; await api(`/api/v1/admin/services/${id}`, { method: "DELETE" }); await refresh(); toast("目标平台已删除"); }
async function deleteMailbox(id) { if (!window.confirm("确定删除该邮箱及其加密凭证？")) return; await api(`/api/v1/admin/mailboxes/${id}`, { method: "DELETE" }); await refresh(); toast("邮箱资源已删除"); }

async function createKey() { const name = document.querySelector("#key-name")?.value.trim(); if (!name) return toast("请输入密钥名称"); const scope = document.querySelector("#key-scope")?.value; const scopes = scope === "read" ? ["orders:read"] : ["orders:read", "orders:write"]; const result = await api("/api/v1/api-keys", { method: "POST", body: JSON.stringify({ name, scopes }) }); showSecret("API Key", result.data.secret); await refresh(); }
async function saveProfile() {
  const result = await api("/api/v1/me", { method: "PUT", body: JSON.stringify({ display_name: document.querySelector("#profile-name")?.value || "" }) });
  state.user = result.data; await render(); toast("账户资料已保存");
}
async function changePassword() {
  const currentPassword = document.querySelector("#current-password")?.value || "";
  const newPassword = document.querySelector("#new-password")?.value || "";
  const confirmPassword = document.querySelector("#confirm-password")?.value || "";
  if (!currentPassword) return toast("请输入当前密码");
  if (newPassword.length < 10) return toast("新密码至少需要 10 位");
  if (newPassword !== confirmPassword) return toast("两次输入的新密码不一致");
  const result = await api("/api/v1/me/password", { method: "PUT", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) });
  state.token = result.data.token; localStorage.setItem("heromail_token", state.token);
  await refresh(); toast("密码已修改，旧会话已全部撤销");
}
async function logout() { try { await api("/api/v1/auth/logout", { method: "POST" }); } finally { state.token = ""; state.user = null; localStorage.removeItem("heromail_token"); redirectToLogin(); } }
async function createPayment() {
  if (state.busyAction === "payment") return;
  const amount = Number(document.querySelector("#topup-amount")?.value);
  const method = document.querySelector("#topup-method")?.value;
  if (!Number.isFinite(amount) || amount < 1 || amount > 100000) { state.paymentError = "充值金额必须在 1 到 100000 元之间"; await render(); return; }
  state.paymentError = "";
  state.busyAction = "payment";
  await render();
  try {
    const result = await api("/api/v1/payment/orders", { method: "POST", body: JSON.stringify({ amount, method, mobile: /Android|iPhone|iPad/i.test(navigator.userAgent) }) });
    if (!result.data?.pay_url) throw new Error("支付链接生成失败，请联系管理员检查支付通道配置");
    location.href = result.data.pay_url;
  } catch (error) {
    state.paymentError = error.message;
    state.busyAction = "";
    toast(error.message);
    await render();
  }
}
async function createWebhook() { const url = document.querySelector("#webhook-url")?.value.trim(); const result = await api("/api/v1/webhooks", { method: "POST", body: JSON.stringify({ url }) }); showSecret("Webhook 签名密钥", result.data.secret); await refresh(); }
function showBalanceEditor(userID) {
  const user = state.users.find(item => item.id === userID);
  if (!user) return toast("用户数据已变化，请刷新后重试");
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal balance-modal"><div class="card-head"><div><h2>调整用户余额</h2><div class="modal-subtitle">${esc(user.email)}</div></div><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body form-grid"><input id="balance-user-id" type="hidden" value="${esc(user.id)}"><input id="balance-current" type="hidden" value="${Number(user.balance)}"><fieldset class="balance-mode"><legend>调整类型</legend><label><input type="radio" name="balance-mode" value="add" checked><span>增加余额</span></label><label><input type="radio" name="balance-mode" value="deduct"><span>扣减余额</span></label></fieldset><label>调整金额<input id="balance-amount" class="field" type="number" min="0.01" step="0.01" placeholder="0.00" inputmode="decimal"></label><label>调整说明（可选）<textarea id="balance-description" class="field" rows="3" maxlength="500" placeholder="例如：订单申诉人工补偿；不填则记录为管理员余额调整"></textarea></label><div id="balance-preview" class="balance-preview"><span>当前余额 <strong>${money(user.balance)}</strong></span><span>调整后 <strong>${money(user.balance)}</strong></span></div><div class="notice">提交后立即记入用户余额，同时写入资金流水和管理审计。</div></div><div class="modal-footer"><button class="ghost-btn" data-action="close-modal">取消</button><button class="primary-btn" data-action="confirm-adjust-balance">确认调整</button></div></div></div>`);
  document.querySelector("#balance-amount")?.focus();
}

function updateBalancePreview() {
  const current = Number(document.querySelector("#balance-current")?.value || 0);
  const amount = Number(document.querySelector("#balance-amount")?.value || 0);
  const mode = document.querySelector('input[name="balance-mode"]:checked')?.value || "add";
  const next = current + (mode === "deduct" ? -amount : amount);
  const preview = document.querySelector("#balance-preview");
  if (preview) preview.innerHTML = `<span>当前余额 <strong>${money(current)}</strong></span><span>调整后 <strong class="${next < 0 ? "danger-text" : ""}">${money(next)}</strong></span>`;
}

async function confirmBalanceAdjustment() {
  const userID = document.querySelector("#balance-user-id")?.value || "";
  const current = Number(document.querySelector("#balance-current")?.value || 0);
  const rawAmount = Number(document.querySelector("#balance-amount")?.value || 0);
  const mode = document.querySelector('input[name="balance-mode"]:checked')?.value || "add";
  const description = document.querySelector("#balance-description")?.value.trim() || "";
  if (!Number.isFinite(rawAmount) || rawAmount <= 0) return toast("请输入大于 0 的调整金额");
  const amount = mode === "deduct" ? -rawAmount : rawAmount;
  if (current + amount < 0) return toast("扣减后余额不能小于 0");
  state.busyAction = "balance";
  const button = document.querySelector('[data-action="confirm-adjust-balance"]');
  if (button) { button.disabled = true; button.textContent = "正在调整…"; }
  try {
    await api(`/api/v1/admin/users/${userID}/balance`, { method: "POST", body: JSON.stringify({ amount, description }) });
    document.querySelector("#secret-modal")?.remove();
    state.adminTabs.users = "ledger"; state.ledgerUserID = userID; state.pagination["admin-ledger"] = { page: 1 };
    history.replaceState({ view: "admin-users" }, "", "/admin/users?tab=ledger");
    await refresh(); toast("余额调整成功，已打开该用户资金流水");
  } finally { state.busyAction = ""; }
}
async function savePaymentProvider() {
  const id = document.querySelector("#pay-provider-id")?.value || "";
  const name = document.querySelector("#pay-provider-name")?.value.trim() || "";
  const type = document.querySelector("#pay-provider-type")?.value;
  if (!name) return toast("请输入服务商名称");
  const config = type === "easypay" ? {
    api_base: document.querySelector("#pay-easypay-api-base")?.value.trim() || "",
    pid: document.querySelector("#pay-easypay-pid")?.value.trim() || "",
    pkey: document.querySelector("#pay-easypay-pkey")?.value.trim() || "",
    channel_id: document.querySelector("#pay-easypay-channel")?.value.trim() || ""
  } : {
    gateway: "https://openapi.alipay.com/gateway.do",
    app_id: document.querySelector("#pay-alipay-app-id")?.value.trim() || "",
    private_key: document.querySelector("#pay-alipay-private-key")?.value.trim() || "",
    public_key: document.querySelector("#pay-alipay-public-key")?.value.trim() || ""
  };
  await api("/api/v1/admin/payment/providers", { method: "POST", body: JSON.stringify({ id, name, type, methods: ["alipay"], enabled: document.querySelector("#pay-provider-enabled")?.checked, priority: Number(document.querySelector("#pay-provider-priority")?.value || 100), config }) });
  document.querySelector("#secret-modal")?.remove(); await refresh(); toast(id ? "支付服务商已更新" : "支付服务商已创建");
}
async function deletePaymentProvider(target) {
  if (!window.confirm(`确定删除支付服务商“${target.dataset.name}”？`)) return;
  await api(`/api/v1/admin/payment/providers/${target.dataset.id}`, { method: "DELETE" });
  await refresh(); toast("支付服务商已删除");
}
async function startMicrosoftOAuth() { const result = await api("/api/v1/admin/mailboxes/oauth/microsoft", { method: "POST", body: JSON.stringify({}) }); location.href = result.data.authorization_url; }
async function checkUpdates() {
  state.version = (await api("/api/v1/admin/system/version")).data;
  await render();
  const current = String(state.version.current_version || "").replace(/^v/i, "");
  const latest = String(state.version.latest_release?.tag || "").replace(/^v/i, "");
  toast(current && current === latest ? "当前已是最新正式版本" : latest ? `发现新版本 ${state.version.latest_release.tag}` : "暂时无法获取最新版本");
}
function showUpgradeConfirmation() {
  const target = state.version?.latest_release?.tag;
  if (!target) return toast("请先检查更新");
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal upgrade-modal"><div class="card-head"><div><h2>确认在线升级</h2><div class="modal-subtitle">${esc(state.version.current_version)} → ${esc(target)}</div></div><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body"><div class="upgrade-confirm-list"><div><span>1</span><p><strong>创建数据库备份</strong>备份验证失败时不会继续升级。</p></div><div><span>2</span><p><strong>切换官方镜像</strong>只使用 <code>ghcr.io/ljunn/heromail:latest</code>。</p></div><div><span>3</span><p><strong>等待健康检查</strong>升级期间页面可能短暂断开，恢复后会继续显示状态。</p></div></div></div><div class="modal-footer"><button class="ghost-btn" data-action="close-modal">暂不升级</button><button class="primary-btn" data-action="confirm-upgrade">开始备份并升级</button></div></div></div>`);
}
async function requestUpgrade() {
  const target = state.version?.latest_release?.tag;
  if (!target) return toast("请先检查更新");
  state.busyAction = "upgrade";
  const button = document.querySelector('[data-action="confirm-upgrade"]');
  if (button) { button.disabled = true; button.textContent = "正在提交…"; }
  try {
    await api("/api/v1/admin/system/upgrade", { method: "POST", headers: { "X-HeroMail-Target-Version": target } });
    document.querySelector("#secret-modal")?.remove();
    toast("升级任务已提交，服务将短暂重启"); startUpgradePolling();
  } catch (error) {
    toast(error.message);
    if (button) { button.disabled = false; button.textContent = "开始备份并升级"; }
  } finally { state.busyAction = ""; }
}
function startUpgradePolling() { clearInterval(state.upgradePolling); state.upgradePolling = setInterval(async () => { try { state.version = (await api("/api/v1/admin/system/version")).data; await render(); if (["success", "failed"].includes(state.version.upgrade.state)) clearInterval(state.upgradePolling); } catch (_) {} }, 3000); }

document.addEventListener("click", async event => {
  if (document.body.classList.contains("mobile-nav-open") && !event.target.closest(".sidebar, .mobile-admin-menu")) {
    document.body.classList.remove("mobile-nav-open");
    document.querySelector(".mobile-admin-menu")?.setAttribute("aria-expanded", "false");
  }
  if (event.target.classList.contains("modal-backdrop")) { event.target.remove(); return; }
  const target = event.target.closest("[data-action]"); if (!target) return;
  const action = target.dataset.action;
  if (action === "role") { location.href = state.role === "admin" ? "/app" : "/admin"; return; }
  if (action === "toggle-admin-menu") {
    const open = document.body.classList.toggle("mobile-nav-open");
    target.setAttribute("aria-expanded", String(open));
    return;
  }
  if (action === "pick-mailbox-file") { if (event.target.id !== "mailbox-file") document.querySelector("#mailbox-file")?.click(); return; }
  if (action === "auth-mode") { location.href = target.dataset.mode === "register" ? "/register" : "/login"; return; }
  if (action === "view") {
    document.body.classList.remove("mobile-nav-open");
    document.querySelector(".mobile-admin-menu")?.setAttribute("aria-expanded", "false");
    await navigate(target.dataset.view); return;
  }
  if (action === "admin-tab") {
    const group = target.dataset.group; const tab = target.dataset.tab;
    if (!state.adminTabs[group]) return;
    state.adminTabs[group] = tab;
    if (group === "users") state.pagination[tab === "ledger" ? "admin-ledger" : "users"] = { page: 1 };
    if (group === "payments") state.pagination[tab === "providers" ? "payment-providers" : "admin-payments"] = { page: 1 };
    if (group === "operations" && tab === "audit") state.pagination.audit = { page: 1 };
    const path = adminRoutes[state.view] || location.pathname;
    history.replaceState({ view: state.view }, "", `${path}?tab=${encodeURIComponent(tab)}`);
    await refresh(); return;
  }
  if (action === "service") {
    if (state.selectedService === target.dataset.service) return;
    state.selectedService = target.dataset.service;
    state.orderError = "";
    await updateApplyView({ selection: true, summary: true });
    return;
  }
  if (action === "create") { await createOrder(); return; }
  if (action === "submit") { state.currentOrder = state.orders.find(order => order.id === target.dataset.order) || state.currentOrder; await mutateOrder("submitted"); return; }
  if (action === "complete") { await mutateOrder("complete"); return; }
  if (action === "cancel") { await mutateOrder("cancel"); return; }
  if (action === "select-order") { await selectOrder(target.dataset.order); return; }
  if (action === "close-order-detail") { state.currentOrder = null; await render(); return; }
  if (action === "reset-order-filters") { state.orderFilters = { status: "", service: "", query: "" }; state.pagination["admin-orders"] = { page: 1 }; await refresh(); return; }
  if (action === "reset-user-order-filters") { state.userOrderFilters = { status: "", service: "", query: "" }; state.pagination.orders = { page: 1 }; await refresh(); return; }
  if (action === "refresh") { await refresh(); return; }
  if (action === "copy") { await copyText(target.dataset.copy || ""); return; }
  if (action === "message") { toast(target.dataset.message || "该功能正在接入中"); }
  if (action === "page") { state.pagination[target.dataset.key] = { ...(state.pagination[target.dataset.key] || {}), page: Number(target.dataset.page) }; await refresh(); return; }
  if (action === "create-key") { await createKey(); return; }
  if (action === "revoke-key") { await api(`/api/v1/api-keys/${target.dataset.id}`, { method: "DELETE" }); await refresh(); return; }
  if (action === "save-profile") { await saveProfile(); return; }
  if (action === "change-password") { await changePassword(); return; }
  if (action === "logout") { await logout(); return; }
  if (action === "create-payment") { await createPayment(); return; }
  if (action === "create-webhook") { await createWebhook(); return; }
  if (action === "delete-webhook") { await api(`/api/v1/webhooks/${target.dataset.id}`, { method: "DELETE" }); await refresh(); return; }
  if (action === "retry-webhook") { await api(`/api/v1/webhook-deliveries/${target.dataset.id}/retry`, { method: "POST" }); await refresh(); return; }
  if (action === "import-mailboxes") { await importMailboxes(); return; }
  if (action === "mailbox-messages") { await showMailboxMessages(target.dataset.id, target.dataset.address); return; }
  if (action === "mailbox-message-page") { await showMailboxMessages(target.dataset.id, target.dataset.address, Number(target.dataset.page)); return; }
  if (action === "delete-mailbox") { await deleteMailbox(target.dataset.id); return; }
  if (action === "verify-mailbox") { await verifyMailbox(target.dataset.id); return; }
  if (action === "edit-service") { showServiceEditor(target.dataset.id || ""); return; }
  if (action === "save-service") { await saveService(); return; }
  if (action === "delete-service") { await deleteService(target.dataset.id); return; }
  if (action === "adjust-balance") { showBalanceEditor(target.dataset.id); return; }
  if (action === "confirm-adjust-balance") { await confirmBalanceAdjustment(); return; }
  if (action === "view-user-ledger") { state.adminTabs.users = "ledger"; state.ledgerUserID = target.dataset.id; state.pagination["admin-ledger"] = { page: 1 }; history.replaceState({ view: "admin-users" }, "", "/admin/users?tab=ledger"); await refresh(); return; }
  if (action === "clear-ledger-user") { state.ledgerUserID = ""; state.pagination["admin-ledger"] = { page: 1 }; await refresh(); return; }
  if (action === "edit-payment-provider") { await showPaymentProviderEditor(target.dataset.id || ""); return; }
  if (action === "save-payment-provider") { await savePaymentProvider(); return; }
  if (action === "delete-payment-provider") { await deletePaymentProvider(target); return; }
  if (action === "microsoft-oauth") { await startMicrosoftOAuth(); return; }
  if (action === "check-updates") { await checkUpdates(); return; }
  if (action === "upgrade") { showUpgradeConfirmation(); return; }
  if (action === "confirm-upgrade") { await requestUpgrade(); return; }
  if (action === "close-modal") { document.querySelector("#secret-modal")?.remove(); return; }
});

document.addEventListener("change", event => {
  if (event.target.id === "mailbox-file") {
    document.querySelector("#mailbox-file-name").textContent = event.target.files?.[0]?.name || "尚未选择文件";
  }
  if (event.target.matches('input[name="balance-mode"]')) updateBalancePreview();
});
document.addEventListener("input", event => {
  if (event.target.id === "balance-amount") updateBalancePreview();
});
document.addEventListener("submit", async event => {
  if (!["order-filters", "user-order-filters"].includes(event.target.dataset.form)) return;
  event.preventDefault();
  if (event.target.dataset.form === "user-order-filters") {
    state.userOrderFilters = { status: document.querySelector("#user-order-status")?.value || "", service: document.querySelector("#user-order-service")?.value || "", query: document.querySelector("#user-order-query")?.value.trim() || "" };
    state.pagination.orders = { page: 1 }; state.currentOrder = null; await refresh(); return;
  }
  state.orderFilters = { status: document.querySelector("#admin-order-status")?.value || "", service: document.querySelector("#admin-order-service")?.value || "", query: document.querySelector("#admin-order-query")?.value.trim() || "" };
  state.pagination["admin-orders"] = { page: 1 };
  state.currentOrder = null;
  await refresh();
});

async function copyText(value) {
  try {
    if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(value); toast("已复制"); return; }
    const input = document.createElement("textarea"); input.value = value; input.style.position = "fixed"; input.style.opacity = "0"; document.body.appendChild(input); input.select();
    if (!document.execCommand("copy")) throw new Error("copy failed");
    input.remove(); toast("已复制");
  } catch (_) { toast("复制失败，请手动选择邮箱"); }
}
document.addEventListener("dragover", event => {
  if (event.target.closest(".mailbox-dropzone")) event.preventDefault();
});
document.addEventListener("drop", event => {
  const zone = event.target.closest(".mailbox-dropzone");
  if (!zone) return;
  event.preventDefault();
  const input = document.querySelector("#mailbox-file");
  if (!input || !event.dataTransfer?.files?.length) return;
  input.files = event.dataTransfer.files;
  input.dispatchEvent(new Event("change", { bubbles: true }));
});

window.addEventListener("popstate", async () => {
  if (!state.user) return;
  state.view = routeView(state.role);
  await refresh();
});
window.addEventListener("keydown", async event => {
  if (event.key !== "Escape") return;
  if (document.body.classList.contains("mobile-nav-open")) {
    document.body.classList.remove("mobile-nav-open");
    document.querySelector(".mobile-admin-menu")?.setAttribute("aria-expanded", "false");
    return;
  }
  if (document.querySelector("#secret-modal")) { document.querySelector("#secret-modal")?.remove(); return; }
  if (state.currentOrder && state.view === "admin-orders") { state.currentOrder = null; await render(); }
});

async function boot() {
  if (!state.token) { redirectToLogin(); return; }
  try {
    state.user = await api("/api/v1/me");
    const wantsAdmin = location.pathname === "/admin" || location.pathname.startsWith("/admin/");
    if (wantsAdmin && state.user.role !== "admin") { location.replace("/app"); return; }
    state.role = wantsAdmin && state.user.role === "admin" ? "admin" : "user";
    state.view = routeView(state.role);
    await refresh();
  } catch (error) { state.token = ""; localStorage.removeItem("heromail_token"); redirectToLogin(); }
}
boot();
