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
  mailboxPool: new URLSearchParams(location.search).get("pool") || "",
  users: [],
  auditLogs: [],
  paymentProviders: [],
  version: null,
  health: null
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
const mobileUserNav = [["apply", "申请", "inbox"], ["current", "任务", "clock"], ["orders", "订单", "receipt"], ["settings", "账户", "userCog"]];

const adminNav = [
  ["运行", [["admin-overview", "运行概览", "dashboard"]]],
  ["资源管理", [["admin-mailboxes", "邮箱池", "database"]]],
  ["业务配置", [["admin-services", "目标平台", "globe"], ["admin-rules", "收码规则", "scan"], ["admin-routing", "调度策略", "route"]]],
  ["订单与用户", [["admin-orders", "注册订单", "receipt"], ["admin-users", "平台用户", "users"], ["admin-ledger", "余额与流水", "ledger"], ["admin-payments", "支付管理", "creditCard"]]],
  ["系统", [["admin-alerts", "告警中心", "bell"], ["admin-audit", "审计日志", "audit"], ["admin-settings", "系统设置", "settings"], ["admin-account", "管理员账户", "shieldUser"]]]
];

const userRoutes = { apply: "/app", current: "/app/tasks", orders: "/app/orders", keys: "/app/developer/keys", webhooks: "/app/developer/webhooks", usage: "/app/usage", balance: "/app/wallet", settings: "/app/account" };
const adminRoutes = { "admin-overview": "/admin", "admin-mailboxes": "/admin/mailboxes", "admin-pools": "/admin/pools", "admin-channels": "/admin/channels", "admin-services": "/admin/services", "admin-rules": "/admin/rules", "admin-routing": "/admin/routing", "admin-orders": "/admin/orders", "admin-users": "/admin/users", "admin-ledger": "/admin/ledger", "admin-payments": "/admin/payments", "admin-alerts": "/admin/alerts", "admin-audit": "/admin/audit", "admin-settings": "/admin/settings", "admin-account": "/admin/account" };

function routeView(role, pathname = location.pathname) {
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
  if (view === "admin-mailboxes" && state.view !== "admin-mailboxes") state.mailboxPool = "";
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
const time = value => { const parsed = value ? new Date(value) : null; return parsed && Number.isFinite(parsed.getTime()) && parsed.getUTCFullYear() > 1 ? parsed.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "-"; };
const connectionLabel = value => ({ auto: "自动", microsoft_graph: "Graph", microsoft_oauth: "Graph OAuth2", imap: "IMAP" }[value] || value || "自动");
const statusMap = { assigned: ["等待提交", "orange"], waiting_code: ["收码中", "blue"], code_received: ["已收码", "green"], completed: ["已完成", "green"], canceled: ["已取消", "red"], expired_refunded: ["已超时退款", "orange"], allocation_failed: ["分配失败", "red"], disputed: ["申诉中", "orange"], pending: ["待支付", "orange"], paid: ["已支付", "blue"], active: ["正常", "green"], disabled: ["已停用", "red"], available: ["可用", "green"], leased: ["租用中", "blue"], cooldown: ["冷却中", "orange"], auth_error: ["授权异常", "red"], blocked: ["已隔离", "red"], verified: ["已验证", "green"], pending_verification: ["待验证", "orange"], failed: ["失败", "red"], idle: ["等待任务", "blue"], backing_up: ["备份中", "orange"], queued: ["已排队", "orange"], updating: ["升级中", "orange"], success: ["成功", "green"] };
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
}

function stat(label, value, note = "") { return `<div class="stat"><div class="stat-label">${label}</div><div class="stat-value">${value}</div><div class="stat-note">${note}</div></div>`; }
function pageHead(title, subtitle, action = "") { return `<div class="page-head"><div><h1>${title}</h1><p>${subtitle}</p></div>${action ? `<div class="head-actions">${action}</div>` : ""}</div>`; }

function serviceCard(service) {
  const selected = service.code === state.selectedService;
  const serviceIcons = { github: "code", openai: "sparkle", discord: "message", telegram: "send" };
  return `<button class="service-card ${selected ? "selected" : ""}" data-action="service" data-service="${esc(service.code)}"><span class="service-logo">${icon(serviceIcons[service.code] || "globe")}</span><span class="service-name">${esc(service.name)}</span><span class="service-desc">${esc(service.description)}</span>${selected ? `<span class="selected-mark">✓</span>` : ""}</button>`;
}

function selectedService() { return state.services.find(service => service.code === state.selectedService) || state.services[0] || { name: "目标平台", price: 0, ttl_seconds: 600, allowed_providers: [] }; }

function renderApply() {
  const service = selectedService();
  const current = state.currentOrder;
  const inventory = Number(service.available_mailboxes || 0); const ttlMinutes = Math.max(1, Math.round(Number(service.ttl_seconds || 600) / 60)); const providers = (service.allowed_providers || []).map(item => item[0].toUpperCase() + item.slice(1)).join(" / ") || "未配置";
  return `<section class="portal-intro"><div><span>邮箱申请</span><h1>选择平台，开始收码任务</h1><p>系统自动分配可用邮箱。你只会看到本次任务需要的邮箱地址和验证码。</p></div><button class="ghost-btn" data-action="view" data-view="orders">查看订单</button></section><section class="portal-service-section"><div class="portal-section-head"><div><h2>目标平台</h2><p>价格、库存和有效期由管理员统一配置</p></div><span class="portal-balance">当前余额 ${money(state.user?.balance)}</span></div><div class="service-grid">${state.services.map(serviceCard).join("") || `<div class="empty">管理员尚未启用目标平台</div>`}</div></section><div class="portal-layout"><section class="card portal-order-tool"><div class="card-head"><h2>确认申请</h2><span class="muted">系统自动选择邮箱</span></div><div class="card-body"><dl class="config-list"><div class="config-row"><dt>目标平台</dt><dd>${esc(service.name)}</dd></div><div class="config-row"><dt>邮箱类型</dt><dd>${esc(providers)}</dd></div><div class="config-row"><dt>可用库存</dt><dd>${inventory} 个</dd></div><div class="config-row"><dt>任务有效期</dt><dd>${ttlMinutes} 分钟</dd></div><div class="config-row"><dt>本次费用</dt><dd>${money(service.price)}</dd></div></dl><button class="primary-btn portal-submit" data-action="create" ${state.busy || !inventory ? "disabled" : ""}>${state.busy ? "正在分配…" : inventory ? "申请邮箱" : "库存不足"}</button><p class="portal-policy">创建时预扣余额。分配失败不扣款，超时未收码自动退款。</p></div></section><section class="card task-card portal-task-tool"><div class="card-head"><h2>当前任务</h2>${current ? statusChip(current.status) : ""}</div><div class="card-body">${current ? renderTask(current) : `<div class="portal-empty-task">${icon("clock")}<strong>暂无进行中的任务</strong><span>申请成功后，邮箱、倒计时和验证码会集中显示在这里。</span></div>`}</div></section></div>${renderRecentOrders()}`;
}

function renderTask(order) {
  const status = order.status;
  const steps = [["assigned", "已分配邮箱"], ["waiting_code", "等待用户提交"], ["code_received", "正在收取验证码"], ["completed", "完成"]];
  const index = status === "completed" ? 4 : status === "code_received" ? 3 : status === "waiting_code" ? 2 : 1;
  const remain = Math.max(0, Math.floor((new Date(order.expires_at) - Date.now()) / 1000));
  return `<div class="steps">${steps.map(([key, label], i) => `<div class="step ${i < index ? "done" : i === index ? "current" : ""}">${label}</div>`).join("")}</div><div class="task-mail"><code>${esc(order.mailbox_address)}</code><button class="link-btn" data-action="copy" data-copy="${esc(order.mailbox_address)}">复制</button></div><div class="notice">请将此邮箱填写到 ${esc(order.service_name)} 注册页面，提交后点击下方按钮。</div>${status === "assigned" ? `<div class="action-row"><button class="primary-btn" data-action="submit" data-order="${order.id}">我已提交注册</button><button class="danger-btn" data-action="cancel" data-order="${order.id}">取消并退款</button></div>` : ""}${status === "waiting_code" ? `<div class="notice warning">正在等待平台注册邮件，剩余 ${Math.floor(remain / 60)}:${String(remain % 60).padStart(2, "0")}。</div>` : ""}${order.code ? `<div class="code-box"><div><div class="code-label">验证码（已收到）</div><div class="code">${esc(order.code)}</div></div><div class="timer">${Math.floor(remain / 60)}:${String(remain % 60).padStart(2, "0")}</div></div><div class="notice success">验证码已提取成功，请在目标平台完成验证。</div>${status === "code_received" ? `<div class="action-row"><button class="primary-btn" data-action="complete" data-order="${order.id}">完成注册</button></div>` : ""}` : ""}`;
}

function renderRecentOrders() {
  const orders = state.orders.slice(0, 5);
  return `<div class="card recent-card"><div class="card-head"><h2>最近订单</h2><button class="link-btn" data-action="view" data-view="orders">查看全部订单 →</button></div><div class="table-wrap">${orders.length ? `<table><thead><tr><th>订单号</th><th>目标平台</th><th>分配邮箱</th><th>状态</th><th>验证码</th><th>费用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${orders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${esc(order.code || "—")}</td><td>${money(order.price)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">详情</button></td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单</div>`}</div></div>`;
}

function renderOrders() {
  const counts = { all: state.orders.length, success: state.orders.filter(order => ["code_received", "completed"].includes(order.status)).length, waiting: state.orders.filter(order => ["assigned", "waiting_code"].includes(order.status)).length, refund: state.orders.filter(order => order.refunded).length };
  return pageHead("订单记录", "查看每次平台注册任务、分配邮箱、验证码和结算结果。", `<button class="primary-btn" data-action="view" data-view="apply">申请邮箱</button>`) + `<div class="stat-grid">${stat("全部订单", counts.all, "累计任务")}${stat("成功收码", counts.success, "已提取验证码")}${stat("等待中", counts.waiting, "有效期内")}${stat("已退款", counts.refund, "超时自动退款")}${stat("今日消费", money(state.orders.reduce((sum, order) => sum + (order.refunded ? 0 : order.price), 0)), "预扣后结算")}</div><div class="card"><div class="filter-bar"><select class="select"><option>全部平台</option>${state.services.map(service => `<option>${esc(service.name)}</option>`).join("")}</select><select class="select"><option>全部状态</option><option>已完成</option><option>收码中</option><option>已退款</option></select><input class="search" placeholder="搜索订单号或邮箱"></div><div class="table-wrap">${state.orders.length ? `<table><thead><tr><th>订单号</th><th>目标平台</th><th>分配邮箱</th><th>状态</th><th>验证码</th><th>费用</th><th>有效期</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.orders.map(order => `<tr class="${state.currentOrder && state.currentOrder.id === order.id ? "selected" : ""}"><td>${esc(order.id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${esc(order.code || "—")}</td><td>${money(order.price)}</td><td>${order.status === "completed" ? "—" : time(order.expires_at)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">详情</button></td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单</div>`}</div></div>${state.currentOrder ? `<div class="card" style="margin-top:16px"><div class="card-head"><h2>订单详情 · ${esc(state.currentOrder.id)}</h2></div><div class="card-body"><div class="timeline">${[["创建订单", state.currentOrder.created_at], ["分配邮箱", state.currentOrder.assigned_at], ["用户已提交", state.currentOrder.submitted_at], ["收到验证码", state.currentOrder.code_received_at], ["完成结算", state.currentOrder.completed_at]].map(([label, value], i) => `<div class="timeline-item ${value ? "done" : ""}"><span class="timeline-dot"></span><div><div class="timeline-title">${label}</div><div class="timeline-time">${time(value)}</div></div></div>`).join("")}</div><div class="notice">邮箱凭证和完整邮件内容不会提供。提交后 10 分钟未收到验证码会自动退款。</div></div></div>` : ""}`;
}

function renderCurrent() {
  return pageHead("当前任务", "正在进行的平台注册任务会在这里显示。", `<button class="ghost-btn" data-action="view" data-view="apply">申请新邮箱</button>`) + `<div class="card"><div class="card-body">${state.currentOrder ? renderTask(state.currentOrder) : `<div class="empty">当前没有进行中的注册任务。</div>`}</div></div>`;
}

function renderAdminOverview() {
  const data = state.overview || {}; const services = state.services; const orders = state.orders.slice(0, 6);
  const maxInventory = Math.max(1, ...services.map(service => Number(service.available_mailboxes || 0)));
  return pageHead("运行概览", "邮箱库存、平台注册任务与收码服务的实时状态。", `<button class="ghost-btn" data-action="refresh">刷新数据</button>`) + `<div class="stat-grid">${stat("可分配邮箱", data.available_mailboxes ?? 0, "全局健康可用")}${stat("活跃租约", data.active_leases ?? 0, "正在进行的任务")}${stat("今日注册订单", data.today_orders ?? 0, "UTC 自然日")}${stat("收码成功率", `${Number(data.success_rate ?? 0).toFixed(2)}%`, "今日实时统计")}${stat("平均收码时间", `${Number(data.average_code_seconds ?? 0).toFixed(1)} 秒`, "今日实时统计")}</div><div class="admin-grid"><div class="card"><div class="card-head"><h2>目标平台库存</h2><button class="link-btn" data-action="view" data-view="admin-services">管理平台 →</button></div><div class="card-body"><div class="bar-list">${services.map(service => { const count = Number(service.available_mailboxes || 0); return `<div class="bar-row"><span>${esc(service.name)}</span><div class="bar-track"><div class="bar-fill" style="width:${Math.round(count / maxInventory * 100)}%"></div></div><strong>${count}</strong></div>`; }).join("") || `<div class="empty">暂无平台</div>`}</div></div></div><div class="card"><div class="card-head"><h2>邮箱渠道健康</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> 可用邮箱 <strong style="margin-left:auto">${data.available_mailboxes ?? 0}</strong></div><div class="status-item"><i class="status-dot ${data.auth_errors ? "orange" : ""}"></i> 授权异常 <strong style="margin-left:auto">${data.auth_errors ?? 0}</strong></div><div class="status-item"><i class="status-dot ${data.blocked_mailboxes ? "red" : ""}"></i> 已隔离 <strong style="margin-left:auto">${data.blocked_mailboxes ?? 0}</strong></div></div></div></div></div><div class="admin-grid"><div class="card"><div class="card-head"><h2>最近注册订单</h2><button class="link-btn" data-action="view" data-view="admin-orders">查看全部 →</button></div><div class="table-wrap">${orders.length ? `<table><thead><tr><th>订单号</th><th>用户</th><th>平台</th><th>邮箱</th><th>状态</th><th>有效期</th></tr></thead><tbody>${orders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.user_id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${order.status === "completed" ? "—" : time(order.expires_at)}</td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单</div>`}</div></div><div class="card"><div class="card-head"><h2>快捷操作</h2></div><div class="card-body"><div class="service-grid"><button class="service-card" data-action="view" data-view="admin-mailboxes"><span class="service-logo">${icon("mail")}</span><span class="service-name">邮箱资源</span><span class="service-desc">检查授权与库存</span></button><button class="service-card" data-action="view" data-view="admin-services"><span class="service-logo">${icon("globe")}</span><span class="service-name">目标平台</span><span class="service-desc">配置规则和价格</span></button></div></div></div></div>`;
}

async function verifyMailbox(id) { await api(`/api/v1/admin/mailboxes/${id}/verify`, { method: "POST" }); await refresh(); toast("邮箱连接验证成功"); }
function renderAdminMailboxesLegacy() {
  const total = state.pagination.mailboxes?.total || 0;
  return pageHead("邮箱资源", "邮箱是平台资产，系统按“邮箱 × 目标平台”决定是否分配。", `<button class="primary-btn" data-action="view" data-view="admin-channels">通过 Microsoft 接入</button>`) + `<div class="stat-grid">${stat("邮箱总数", total, "已接入资产")}${stat("当页可用", state.mailboxes.filter(mailbox => mailbox.state === "available").length, "健康分 ≥ 60")}${stat("当页租用中", state.mailboxes.filter(mailbox => mailbox.state === "leased").length, "注册任务")}${stat("当页授权异常", state.mailboxes.filter(mailbox => mailbox.state === "auth_error").length, "需要重新授权")}${stat("当页隔离", state.mailboxes.filter(mailbox => mailbox.state === "blocked").length, "人工处理")}</div><div class="card"><div class="table-wrap"><table><thead><tr><th>邮箱</th><th>供应商</th><th>邮箱池</th><th>连接方式</th><th>验证状态</th><th>已注册平台</th><th>OAuth 有效期</th><th>健康分</th><th>最近验证</th><th>最近收信</th><th>操作</th></tr></thead><tbody>${state.mailboxes.map(mailbox => `<tr><td>${esc(mailbox.address)}</td><td>${esc(mailbox.provider)}</td><td>${esc(mailbox.pool)}</td><td>${mailbox.connection_method === "microsoft_oauth" ? "Microsoft OAuth2" : esc(mailbox.connection_method || "—")}</td><td>${statusChip(mailbox.verification_status || mailbox.state)}${mailbox.verification_error ? `<div class="muted">${esc(mailbox.verification_error)}</div>` : ""}</td><td>${esc((mailbox.registered_platforms || []).join(", ") || "—")}</td><td>${mailbox.oauth_valid_until ? new Date(mailbox.oauth_valid_until).toLocaleString("zh-CN") : "—"}</td><td>${mailbox.health_score}/100</td><td>${time(mailbox.last_verified_at)}</td><td>${time(mailbox.last_received_at)}</td><td><div class="table-actions"><button class="link-btn" data-action="verify-mailbox" data-id="${esc(mailbox.id)}">验证</button><button class="link-btn danger-text" data-action="delete-mailbox" data-id="${esc(mailbox.id)}">删除</button></div></td></tr>`).join("") || `<tr><td colspan="11" class="empty">尚未接入邮箱</td></tr>`}</tbody></table></div>${renderPager("mailboxes")}</div>`;
}

function renderAdminPools() {
  const rows = state.pools.map(pool => `<tr><td><strong>${esc(pool.name)}</strong></td><td>${esc(pool.provider)}</td><td>${pool.mailbox_count}</td><td>${pool.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><div class="table-actions"><button class="link-btn" data-action="view-pool" data-pool="${esc(pool.name)}">查看邮箱</button><button class="link-btn danger-text" data-action="delete-pool" data-id="${esc(pool.id)}">删除</button></div></td></tr>`).join("");
  return pageHead("邮箱池", "先选邮箱池，再查看该池中的邮箱列表。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>名称</th><th>供应商</th><th>邮箱数</th><th>状态</th><th>操作</th></tr></thead><tbody>${rows || `<tr><td colspan="5" class="empty">暂无邮箱池</td></tr>`}</tbody></table></div>${renderPager("pools")}</div><div class="card"><div class="card-head"><h2>新增邮箱池</h2></div><div class="card-body form-grid"><label>名称<input id="pool-name" class="field"></label><label>供应商<select id="pool-provider" class="field"><option value="outlook">Outlook</option><option value="hotmail">Hotmail</option></select></label><label>地区<input id="pool-region" class="field" value="global"></label><label>日限额<input id="pool-limit" class="field" type="number" min="1" value="100"></label><label>冷却秒数<input id="pool-cooldown" class="field" type="number" min="0" value="60"></label><button class="primary-btn" data-action="create-pool">保存邮箱池</button></div></div>`;
}

function renderAdminServices() {
  return pageHead("目标平台", "配置邮箱类型、收件规则、价格与任务有效期。", `<button class="primary-btn" data-action="edit-service">新建目标平台</button>`) + `<div class="card"><div class="table-wrap"><table><thead><tr><th>平台</th><th>平台代码</th><th>状态</th><th>可分配</th><th>租用中</th><th>已使用</th><th>单价</th><th>有效期</th><th>操作</th></tr></thead><tbody>${state.services.map(service => `<tr><td><strong>${esc(service.name)}</strong><div class="muted">${esc(service.description)}</div></td><td><code>${esc(service.code)}</code></td><td>${service.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td>${service.available_mailboxes ?? 0}</td><td>${service.leased_mailboxes ?? 0}</td><td>${service.consumed_mailboxes ?? 0}</td><td>${money(service.price)}</td><td>${Math.round(service.ttl_seconds / 60)} 分钟</td><td><button class="link-btn" data-action="edit-service" data-id="${esc(service.id)}">编辑</button> <button class="link-btn danger-text" data-action="delete-service" data-id="${esc(service.id)}">删除</button></td></tr>`).join("") || `<tr><td colspan="9" class="empty">暂无目标平台</td></tr>`}</tbody></table></div>${renderPager("admin-services")}</div>`;
}

function renderAdminOrders() {
  const total = state.pagination["admin-orders"]?.total || 0;
  return pageHead("注册订单", "监控平台注册全链路、收码状态、退款和邮箱平台占用。", `<button class="ghost-btn" data-action="refresh">刷新数据</button>`) + `<div class="stat-grid">${stat("订单总数", total, "全部记录")}${stat("当页已收码", state.orders.filter(order => order.code).length, "验证码已匹配")}${stat("当页收码中", state.orders.filter(order => order.status === "waiting_code").length, "等待邮件")}${stat("当页已退款", state.orders.filter(order => order.refunded).length, "超时或取消")}${stat("当页异常", state.orders.filter(order => ["allocation_failed", "disputed"].includes(order.status)).length, "需要人工处理")}</div><div class="card"><div class="table-wrap"><table><thead><tr><th>订单号</th><th>用户</th><th>目标平台</th><th>分配邮箱</th><th>状态</th><th>验证码</th><th>费用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.orders.map(order => `<tr class="${state.currentOrder && state.currentOrder.id === order.id ? "selected" : ""}"><td>${esc(order.id)}</td><td>${esc(order.user_id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${esc(order.code || "—")}</td><td>${money(order.price)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">详情</button></td></tr>`).join("") || `<tr><td colspan="9" class="empty">暂无订单</td></tr>`}</tbody></table></div>${renderPager("admin-orders")}</div>${state.currentOrder ? `<div class="card" style="margin-top:16px"><div class="card-head"><h2>订单详情 · ${esc(state.currentOrder.id)}</h2><span>${statusChip(state.currentOrder.status)}</span></div><div class="card-body drawer-grid"><div class="timeline">${[["创建订单", state.currentOrder.created_at], ["分配邮箱", state.currentOrder.assigned_at], ["用户已提交", state.currentOrder.submitted_at], ["收到验证码", state.currentOrder.code_received_at], ["完成结算", state.currentOrder.completed_at]].map(([label, value]) => `<div class="timeline-item ${value ? "done" : ""}"><span class="timeline-dot"></span><div><div class="timeline-title">${label}</div><div class="timeline-time">${time(value)}</div></div></div>`).join("")}</div><div><dl class="config-list"><div class="config-row"><dt>目标平台</dt><dd>${esc(state.currentOrder.service_name)}</dd></div><div class="config-row"><dt>分配邮箱</dt><dd>${esc(state.currentOrder.mailbox_address)}</dd></div><div class="config-row"><dt>验证码</dt><dd>${esc(state.currentOrder.code || "—")}</dd></div><div class="config-row"><dt>费用</dt><dd>${money(state.currentOrder.price)}</dd></div></dl></div></div></div>` : ""}`;
}

function renderPager(key) {
  const page = state.pagination[key]; if (!page || page.total_pages <= 1) return "";
  return `<div class="pager"><button class="ghost-btn" data-action="page" data-key="${key}" data-page="${page.page - 1}" ${page.page <= 1 ? "disabled" : ""}>上一页</button><span>第 ${page.page} / ${page.total_pages} 页 · 共 ${page.total} 条</span><button class="ghost-btn" data-action="page" data-key="${key}" data-page="${page.page + 1}" ${page.page >= page.total_pages ? "disabled" : ""}>下一页</button></div>`;
}

function renderAPIKeys() {
  return pageHead("API 密钥", "管理用于服务端调用订单接口的访问密钥。") + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>密钥列表</h2></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>前缀</th><th>权限</th><th>最后使用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.apiKeys.map(key => `<tr><td>${esc(key.name)}</td><td><code>${esc(key.prefix)}…</code></td><td>${esc(key.scopes.join(", "))}</td><td>${time(key.last_used_at)}</td><td>${time(key.created_at)}</td><td><button class="link-btn danger-text" data-action="revoke-key" data-id="${key.id}">吊销</button></td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无 API Key</td></tr>`}</tbody></table></div>${renderPager("keys")}</div><div class="card"><div class="card-head"><h2>创建密钥</h2></div><div class="card-body form-grid"><label>密钥名称<input id="key-name" class="field" placeholder="生产环境"></label><label>权限范围<select id="key-scope" class="field"><option value="orders">订单读写</option><option value="read">仅订单读取</option></select></label><button class="primary-btn" data-action="create-key">创建 API Key</button><div class="notice">密钥明文只显示一次，服务端仅保存 SHA-256 哈希。</div></div></div></div>`;
}

function renderUsage() {
  return pageHead("用量与账单", "查看订单扣费、退款、支付充值和人工调整记录。") + `<div class="stat-grid">${stat("当前余额", money(state.user.balance), "可用额度")}${stat("流水数量", state.pagination.ledgers?.total || 0, "全部资金变动")}</div><div class="card"><div class="table-wrap"><table><thead><tr><th>流水号</th><th>类型</th><th>变动金额</th><th>变动后余额</th><th>关联订单</th><th>说明</th><th>时间</th></tr></thead><tbody>${state.ledgers.map(item => `<tr><td>${esc(item.id)}</td><td>${esc(item.type)}</td><td class="${item.amount >= 0 ? "positive" : "negative"}">${item.amount >= 0 ? "+" : ""}${money(item.amount)}</td><td>${money(item.balance_after)}</td><td>${esc(item.order_id || item.payment_order_id || "—")}</td><td>${esc(item.description)}</td><td>${time(item.created_at)}</td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无资金流水</td></tr>`}</tbody></table></div>${renderPager("ledgers")}</div>`;
}

function renderBalance() {
  return pageHead("余额充值", "通过管理员配置的支付宝官方或易支付通道充值。") + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>创建充值订单</h2><strong>${money(state.user.balance)}</strong></div><div class="card-body form-grid"><label>充值金额<input id="topup-amount" class="field" type="number" min="1" max="100000" step="0.01" value="50"></label><label>支付方式<select id="topup-method" class="field">${state.paymentMethods.map(method => `<option value="${esc(method)}">${method === "alipay" ? "支付宝" : esc(method)}</option>`).join("")}</select></label><button class="primary-btn" data-action="create-payment" ${state.paymentMethods.length ? "" : "disabled"}>前往支付</button>${state.paymentMethods.length ? "" : `<div class="notice warning">管理员尚未启用支付服务商。</div>`}</div></div><div class="card"><div class="card-head"><h2>充值记录</h2></div><div class="table-wrap"><table><thead><tr><th>支付单</th><th>通道</th><th>金额</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.paymentOrders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.provider_name)}</td><td>${money(order.amount)}</td><td>${statusChip(order.status)}</td><td>${time(order.created_at)}</td><td>${order.status === "pending" && order.pay_url ? `<a class="link-btn" href="${esc(order.pay_url)}" target="_blank" rel="noopener">继续支付</a>` : "—"}</td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无充值记录</td></tr>`}</tbody></table></div>${renderPager("payments")}</div></div>`;
}

function renderAccountSettings(title, subtitle) {
  return pageHead(title, subtitle) + `<div class="account-grid"><div class="card"><div class="card-head"><h2>账户资料</h2>${icon("userCog", "section-icon")}</div><div class="card-body form-grid"><label>登录邮箱<input class="field" value="${esc(state.user.email)}" disabled></label><label>显示名称<input id="profile-name" class="field" value="${esc(state.user.display_name || "")}" maxlength="120" autocomplete="name"></label><button class="primary-btn" data-action="save-profile">保存账户资料</button></div></div><div class="card"><div class="card-head"><h2>登录密码</h2>${icon("shieldUser", "section-icon")}</div><div class="card-body form-grid"><label>当前密码<input id="current-password" class="field" type="password" maxlength="128" autocomplete="current-password"></label><label>新密码<input id="new-password" class="field" type="password" minlength="10" maxlength="128" autocomplete="new-password" placeholder="至少 10 位"></label><label>确认新密码<input id="confirm-password" class="field" type="password" minlength="10" maxlength="128" autocomplete="new-password"></label><button class="primary-btn" data-action="change-password">修改登录密码</button><div class="notice">修改后会撤销该账户的全部旧会话，当前浏览器将自动切换到新会话。</div></div></div></div><button class="danger-btn account-logout" data-action="logout">退出登录</button>`;
}

function renderSettings() {
  return renderAccountSettings("个人设置", "维护账户资料和登录密码。");
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

function renderAdminPoolsLegacy() {
  return pageHead("邮箱池", "配置邮箱供应商、地区、日限额和冷却时间。") + `<div class="admin-grid"><div class="card"><div class="table-wrap"><table><thead><tr><th>名称</th><th>供应商</th><th>地区</th><th>邮箱数</th><th>日限额</th><th>冷却</th><th>状态</th><th>操作</th></tr></thead><tbody>${state.pools.map(pool => `<tr><td>${esc(pool.name)}</td><td>${esc(pool.provider)}</td><td>${esc(pool.region || "—")}</td><td>${pool.mailbox_count}</td><td>${pool.daily_limit}</td><td>${pool.cooldown_seconds} 秒</td><td>${pool.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><button class="link-btn danger-text" data-action="delete-pool" data-id="${esc(pool.id)}">删除</button></td></tr>`).join("") || `<tr><td colspan="8" class="empty">暂无邮箱池</td></tr>`}</tbody></table></div>${renderPager("pools")}</div><div class="card"><div class="card-head"><h2>新增邮箱池</h2></div><div class="card-body form-grid"><label>名称<input id="pool-name" class="field"></label><label>供应商<select id="pool-provider" class="field"><option value="outlook">Outlook</option><option value="hotmail">Hotmail</option></select></label><label>地区<input id="pool-region" class="field" value="global"></label><label>日限额<input id="pool-limit" class="field" type="number" min="1" value="100"></label><label>冷却秒数<input id="pool-cooldown" class="field" type="number" min="0" value="60"></label><button class="primary-btn" data-action="create-pool">保存邮箱池</button></div></div></div>`;
}

async function importMailboxes() {
  const file = document.querySelector("#mailbox-file")?.files?.[0];
  const pool = document.querySelector("#mailbox-import-pool")?.value || state.mailboxPool;
  if (!file) return toast("请选择 TXT 或 CSV 文件");
  if (!pool) return toast("请选择邮箱池");
  const form = new FormData(); form.append("file", file);
  const result = await api(`/api/v1/admin/mailboxes/import?pool=${encodeURIComponent(pool)}`, { method: "POST", body: form });
  state.mailboxPool = pool;
  history.replaceState({}, "", `/admin/mailboxes?pool=${encodeURIComponent(pool)}`);
  await refresh();
  toast(`导入完成：${result.data.imported} 个成功，${result.data.failed} 个失败`);
}

function renderAdminMailboxes() {
  const total = state.pagination.mailboxes?.total || 0;
  const poolOptions = state.pools.map(pool => `<option value="${esc(pool.name)}" ${state.mailboxPool === pool.name ? "selected" : ""}>${esc(pool.name)} · ${esc(pool.provider)}</option>`).join("");
  const poolLabel = state.mailboxPool ? ` · ${esc(state.mailboxPool)}` : "";
  const rows = state.mailboxes.map(mailbox => `<tr><td>${esc(mailbox.address)}</td><td>${esc(mailbox.provider)}</td><td>${esc(mailbox.connection_method || "auto")}</td><td>${statusChip(mailbox.verification_status || mailbox.state)}${mailbox.verification_error ? `<div class="muted">${esc(mailbox.verification_error)}</div>` : ""}</td><td>${esc((mailbox.registered_platforms || []).join(", ") || "—")}</td><td>${mailbox.health_score}/100</td><td>${time(mailbox.last_verified_at)}</td><td><div class="table-actions"><button class="link-btn" data-action="verify-mailbox" data-id="${esc(mailbox.id)}">验证</button><button class="link-btn danger-text" data-action="delete-mailbox" data-id="${esc(mailbox.id)}">删除</button></div></td></tr>`).join("");
  return pageHead(`邮箱列表${poolLabel}`, "导入后系统自动验证，Graph 优先，失败时回退 IMAP。", `<button class="ghost-btn" data-action="view" data-view="admin-pools">返回邮箱池</button>`) + `<div class="card"><div class="card-head"><h2>导入邮箱</h2><span class="muted">按行流式读取，支持 TXT / CSV / JSON Lines</span></div><div class="card-body form-grid"><div class="form-columns"><label>邮箱池<select id="mailbox-import-pool" class="field">${poolOptions || `<option value="">请先创建邮箱池</option>`}</select></label><label>文件<input id="mailbox-file" class="field" type="file" accept=".txt,.csv,text/plain,text/csv"></label></div><button class="primary-btn" data-action="import-mailboxes" ${state.pools.length ? "" : "disabled"}>导入并自动验证</button></div></div><div class="stat-grid">${stat("邮箱总数", total, "当前邮箱池")}${stat("当页可用", state.mailboxes.filter(mailbox => mailbox.state === "available").length, "验证通过")}${stat("当页待验证", state.mailboxes.filter(mailbox => mailbox.verification_status === "pending_verification").length, "后台队列处理中")}${stat("当页已注册平台", state.mailboxes.filter(mailbox => (mailbox.registered_platforms || []).length > 0).length, "按平台占用")}</div><div class="card"><div class="table-wrap"><table><thead><tr><th>邮箱</th><th>供应商</th><th>连接方式</th><th>验证状态</th><th>已注册平台</th><th>健康分</th><th>最近验证</th><th>操作</th></tr></thead><tbody>${rows || `<tr><td colspan="8" class="empty">尚未接入邮箱</td></tr>`}</tbody></table></div>${renderPager("mailboxes")}</div>`;
}

function renderAdminUsers() {
  return pageHead("平台用户", "管理账户状态、角色和余额。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>邮箱</th><th>显示名称</th><th>角色</th><th>状态</th><th>余额</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.users.map(user => `<tr><td>${esc(user.email)}</td><td>${esc(user.display_name || "—")}</td><td>${esc(user.role)}</td><td>${statusChip(user.status)}</td><td>${money(user.balance)}</td><td>${time(user.created_at)}</td><td><button class="link-btn" data-action="adjust-balance" data-id="${user.id}" data-email="${esc(user.email)}">调整余额</button></td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无用户</td></tr>`}</tbody></table></div>${renderPager("users")}</div>`;
}

function renderAdminLedger() { return pageHead("余额与流水", "查看全平台资金变动与关联业务。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>流水号</th><th>类型</th><th>金额</th><th>余额</th><th>关联业务</th><th>说明</th><th>时间</th></tr></thead><tbody>${state.ledgers.map(item => `<tr><td>${esc(item.id)}</td><td>${esc(item.type)}</td><td>${money(item.amount)}</td><td>${money(item.balance_after)}</td><td>${esc(item.order_id || item.payment_order_id || "—")}</td><td>${esc(item.description)}</td><td>${time(item.created_at)}</td></tr>`).join("") || `<tr><td colspan="7" class="empty">暂无流水</td></tr>`}</tbody></table></div>${renderPager("admin-ledger")}</div>`; }

function renderAdminAudit() { return pageHead("审计日志", "追踪管理操作、资金变动、配置和升级行为。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>操作者</th><th>动作</th><th>资源</th><th>详情</th><th>IP</th><th>时间</th></tr></thead><tbody>${state.auditLogs.map(item => `<tr><td>${esc(item.actor_id)}</td><td><code>${esc(item.action)}</code></td><td>${esc(item.resource_type)} · ${esc(item.resource_id)}</td><td>${esc(item.detail)}</td><td>${esc(item.ip || "—")}</td><td>${time(item.created_at)}</td></tr>`).join("") || `<tr><td colspan="6" class="empty">暂无审计记录</td></tr>`}</tbody></table></div>${renderPager("audit")}</div>`; }

function renderAdminPayments() {
  const providerRows = state.paymentProviders.map(item => `<tr><td><strong>${esc(item.name)}</strong></td><td>${item.type === "easypay" ? "易支付" : "支付宝官方"}</td><td>${esc(item.methods.map(method => method === "alipay" ? "支付宝" : method).join(", "))}</td><td>${item.priority}</td><td>${item.enabled ? `<span class="chip green">启用</span>` : `<span class="chip red">停用</span>`}</td><td><div class="table-actions"><button class="link-btn" data-action="edit-payment-provider" data-id="${esc(item.id)}">编辑</button><button class="link-btn danger-text" data-action="delete-payment-provider" data-id="${esc(item.id)}" data-name="${esc(item.name)}">删除</button></div></td></tr>`).join("");
  const orderRows = state.paymentOrders.map(item => `<tr><td>${esc(item.id)}</td><td>${esc(item.user_id)}</td><td>${esc(item.provider_name)}</td><td>${money(item.amount)}</td><td>${statusChip(item.status)}</td><td>${esc(item.provider_trade_no || "—")}</td><td>${time(item.created_at)}</td></tr>`).join("");
  return pageHead("支付管理", "分别配置易支付和支付宝官方通道，并管理充值订单。", `<button class="primary-btn" data-action="edit-payment-provider">新增服务商</button>`) + `<div class="card"><div class="card-head"><h2>支付服务商</h2><span class="muted">敏感凭证使用 AES-256-GCM 加密保存</span></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>类型</th><th>支付方式</th><th>优先级</th><th>状态</th><th>操作</th></tr></thead><tbody>${providerRows || `<tr><td colspan="6" class="empty">暂无支付服务商</td></tr>`}</tbody></table></div>${renderPager("payment-providers")}</div><div class="provider-guidance"><div><span class="provider-badge easypay">易</span><strong>易支付</strong><p>需要 API 地址、商户 ID（PID）和商户密钥（PKey）。</p></div><div><span class="provider-badge alipay">支</span><strong>支付宝官方</strong><p>官方网关已内置，只需 AppID、应用私钥和支付宝公钥。</p></div></div><div class="card" style="margin-top:16px"><div class="card-head"><h2>充值订单</h2></div><div class="table-wrap"><table><thead><tr><th>支付单</th><th>用户</th><th>服务商</th><th>金额</th><th>状态</th><th>上游流水</th><th>创建时间</th></tr></thead><tbody>${orderRows || `<tr><td colspan="7" class="empty">暂无支付订单</td></tr>`}</tbody></table></div>${renderPager("admin-payments")}</div>`;
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
  const upgradeLabel = upgradeAvailable ? `升级到 ${esc(release.tag)}` : hasRelease ? "已是最新版本" : "暂无可用版本";
  return pageHead("系统设置", "检查 GitHub 正式版本并从管理后台完成升级。", `<button class="ghost-btn" data-action="check-updates">检查更新</button>`) + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>版本信息</h2>${statusChip(version.upgrade.state)}</div><div class="card-body"><dl class="config-list"><div class="config-row"><dt>当前版本</dt><dd>${esc(version.current_version)}</dd></div><div class="config-row"><dt>最新版本</dt><dd>${esc(release.tag || "暂未获取")}</dd></div><div class="config-row"><dt>提交</dt><dd><code>${esc(version.commit)}</code></dd></div><div class="config-row"><dt>构建时间</dt><dd>${esc(version.build_time)}</dd></div><div class="config-row"><dt>升级状态</dt><dd>${esc(version.upgrade.message)}</dd></div></dl></div></div><div class="card"><div class="card-head"><h2>在线升级</h2>${icon("activity", "section-icon")}</div><div class="card-body form-grid"><button class="primary-btn" data-action="upgrade" ${version.online_upgrade_enabled && upgradeAvailable ? "" : "disabled"}>${upgradeLabel}</button><div class="notice warning">升级前会自动创建并校验 PostgreSQL 备份。确认后仅拉取 GitHub Release 工作流发布的 <code>ghcr.io/ljunn/heromail:latest</code>，不会更改 PostgreSQL 和 Redis 数据卷。</div></div></div></div><div class="card" style="margin-top:16px"><div class="card-head"><h2>最新版本日志 · ${esc(release.tag || "暂无")}</h2>${release.url ? `<a class="link-btn" href="${esc(release.url)}" target="_blank" rel="noopener">查看 GitHub Release</a>` : ""}</div><div class="card-body">${hasRelease ? `<pre class="release-notes">${esc(release.notes || "该版本未提供更新日志。")}</pre>` : `<div class="empty">无法获取 GitHub 最新正式版本，已禁用升级按钮。</div>`}</div></div>`;
}

function renderAdminChannels() { return pageHead("接入渠道", "通过 Microsoft OAuth2 接入 Outlook 和 Hotmail。") + `<div class="admin-grid"><div class="card"><div class="card-head"><h2>Microsoft Graph OAuth2</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> OAuth 授权流程 <span class="status-ok">服务端验证</span></div><div class="status-item"><i class="status-dot"></i> Token 存储 <span class="status-ok">AES-256-GCM</span></div><div class="status-item"><i class="status-dot"></i> 已接入邮箱 <strong style="margin-left:auto">${state.pagination.mailboxes?.total || 0}</strong></div></div></div></div><div class="card"><div class="card-head"><h2>连接邮箱</h2></div><div class="card-body form-grid"><label>目标邮箱池<select id="oauth-pool" class="field">${state.pools.map(pool => `<option value="${esc(pool.name)}">${esc(pool.name)} · ${esc(pool.provider)}</option>`).join("")}</select></label><button class="primary-btn" data-action="microsoft-oauth" ${state.pools.length ? "" : "disabled"}>连接 Microsoft 邮箱</button>${state.pools.length ? `<div class="notice">OAuth 完成后会自动识别 Outlook/Hotmail 地址并加入所选邮箱池。</div>` : `<div class="notice warning">请先在“邮箱池”页面创建邮箱池。</div>`}</div></div></div>`; }

function renderAdminRules() { return pageHead("收码规则", "发件人域名、主题关键词和验证码正则按目标平台隔离。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>平台</th><th>发件人域名</th><th>主题关键词</th><th>验证码正则</th><th>有效期</th></tr></thead><tbody>${state.services.map(item => `<tr><td>${esc(item.name)}</td><td>${esc(item.sender_domains.join(", "))}</td><td>${esc(item.subject_keywords.join(", "))}</td><td><code>${esc(item.regex)}</code></td><td>${Math.round(item.ttl_seconds / 60)} 分钟</td></tr>`).join("")}</tbody></table></div>${renderPager("admin-services")}</div>`; }

function renderAdminRouting() { return pageHead("调度策略", "订单分配同时检查邮箱池、供应商、健康分、租约和平台占用。") + `<div class="card"><div class="table-wrap"><table><thead><tr><th>平台</th><th>允许供应商</th><th>可分配</th><th>租用中</th><th>已消费</th><th>价格</th></tr></thead><tbody>${state.services.map(item => `<tr><td>${esc(item.name)}</td><td>${esc(item.allowed_providers.join(", "))}</td><td>${item.available_mailboxes ?? "—"}</td><td>${item.leased_mailboxes ?? "—"}</td><td>${item.consumed_mailboxes ?? "—"}</td><td>${money(item.price)}</td></tr>`).join("")}</tbody></table></div></div>`; }

function renderAdminAlerts() { const overview = state.overview || {}; const alerts = []; if (!overview.available_mailboxes) alerts.push(["库存告警", "没有可分配邮箱", "red"]); if (overview.auth_errors) alerts.push(["授权异常", `${overview.auth_errors} 个邮箱需要重新授权`, "orange"]); if (overview.blocked_mailboxes) alerts.push(["邮箱隔离", `${overview.blocked_mailboxes} 个邮箱已隔离`, "red"]); return pageHead("告警中心", "根据库存、授权和邮箱状态生成运行告警。") + `<div class="card"><div class="card-body status-list">${alerts.map(item => `<div class="status-item"><i class="status-dot ${item[2]}"></i><strong>${item[0]}</strong><span class="muted">${item[1]}</span></div>`).join("") || `<div class="empty">当前没有运行告警</div>`}</div></div>`; }


function render() {
  if (!state.user) { redirectToLogin(); return Promise.resolve(); }
  document.body.classList.toggle("admin-shell", state.role === "admin");
  document.body.classList.toggle("user-shell", state.role !== "admin");
  document.body.classList.remove("auth-mode");
  renderNav();
  const views = { apply: renderApply, current: renderCurrent, orders: () => renderOrders() + renderPager("orders"), docs: renderDocs, keys: renderAPIKeys, webhooks: renderWebhooks, usage: renderUsage, balance: renderBalance, settings: renderSettings, "admin-overview": renderAdminOverview, "admin-mailboxes": renderAdminMailboxes, "admin-pools": renderAdminPools, "admin-channels": renderAdminChannels, "admin-services": renderAdminServices, "admin-rules": renderAdminRules, "admin-routing": renderAdminRouting, "admin-orders": renderAdminOrders, "admin-users": renderAdminUsers, "admin-ledger": renderAdminLedger, "admin-payments": renderAdminPayments, "admin-alerts": renderAdminAlerts, "admin-audit": renderAdminAudit, "admin-settings": renderAdminSettings, "admin-account": renderAdminAccount };
  document.querySelector("#content").innerHTML = (views[state.view] || views.apply)();
  document.querySelector("#balance").textContent = `余额 ${money(state.user.balance)}`;
  document.querySelector(".avatar").textContent = (state.user.display_name || state.user.email || "U").slice(0, 1).toUpperCase();
  if (state.version?.current_version) document.querySelector("#app-version").textContent = state.version.current_version;
  return Promise.resolve();
}

function rememberPage(key, body) { if (body.pagination) state.pagination[key] = body.pagination; return body.data || []; }
function requestedPage(key) { return state.pagination[key]?.page || 1; }
async function loadUser() {
  const page = state.pagination.orders?.page || 1;
  const [me, services, orders] = await Promise.all([api("/api/v1/me"), api("/api/v1/services?page=1&page_size=100"), api(`/api/v1/orders?page=${page}&page_size=20`)]);
  state.user = me; state.services = services.data || []; state.orders = rememberPage("orders", orders); if (!state.services.some(service => service.code === state.selectedService) && state.services[0]) state.selectedService = state.services[0].code;
  if (!state.currentOrder || !state.orders.some(order => order.id === state.currentOrder.id)) state.currentOrder = state.orders.find(order => ["assigned", "waiting_code", "code_received"].includes(order.status)) || null;
  if (["keys", "usage", "balance", "webhooks"].includes(state.view)) await loadUserModule(state.view);
}
async function loadUserModule(view) {
  if (view === "keys") state.apiKeys = rememberPage("keys", await api(`/api/v1/api-keys?page=${requestedPage("keys")}&page_size=20`));
  if (view === "usage") state.ledgers = rememberPage("ledgers", await api(`/api/v1/wallet/ledgers?page=${requestedPage("ledgers")}&page_size=20`));
  if (view === "balance") { state.paymentMethods = (await api("/api/v1/payment/methods")).data || []; state.paymentOrders = rememberPage("payments", await api(`/api/v1/payment/orders?page=${requestedPage("payments")}&page_size=20`)); }
  if (view === "webhooks") { state.webhookEndpoints = rememberPage("webhooks", await api(`/api/v1/webhooks?page=${requestedPage("webhooks")}&page_size=20`)); state.webhookDeliveries = rememberPage("webhook-deliveries", await api(`/api/v1/webhook-deliveries?page=${requestedPage("webhook-deliveries")}&page_size=20`)); }
}
async function loadAdmin() {
  const mailboxFilter = state.mailboxPool ? `&pool=${encodeURIComponent(state.mailboxPool)}` : "";
  const [me, overview, services, mailboxes, orders] = await Promise.all([api("/api/v1/me"), api("/api/v1/admin/overview"), api(`/api/v1/admin/services?page=${requestedPage("admin-services")}&page_size=20`), api(`/api/v1/admin/mailboxes?page=${requestedPage("mailboxes")}&page_size=20${mailboxFilter}`), api(`/api/v1/admin/orders?page=${requestedPage("admin-orders")}&page_size=20`)]);
  state.user = me; state.overview = overview.data; state.services = rememberPage("admin-services", services); state.mailboxes = rememberPage("mailboxes", mailboxes); state.orders = rememberPage("admin-orders", orders);
  if (["admin-pools", "admin-mailboxes", "admin-channels"].includes(state.view)) state.pools = rememberPage("pools", await api(`/api/v1/admin/mailbox-pools?page=${requestedPage("pools")}&page_size=20`));
  if (state.view === "admin-users") state.users = rememberPage("users", await api(`/api/v1/admin/users?page=${requestedPage("users")}&page_size=20`));
  if (state.view === "admin-ledger") state.ledgers = rememberPage("admin-ledger", await api(`/api/v1/admin/wallet/ledgers?page=${requestedPage("admin-ledger")}&page_size=20`));
  if (state.view === "admin-audit") state.auditLogs = rememberPage("audit", await api(`/api/v1/admin/audit-logs?page=${requestedPage("audit")}&page_size=20`));
  if (state.view === "admin-payments") { state.paymentProviders = rememberPage("payment-providers", await api(`/api/v1/admin/payment/providers?page=${requestedPage("payment-providers")}&page_size=20`)); state.paymentOrders = rememberPage("admin-payments", await api(`/api/v1/admin/payment/orders?page=${requestedPage("admin-payments")}&page_size=20`)); }
  if (state.view === "admin-settings") state.version = (await api("/api/v1/admin/system/version")).data;
}

async function refresh() { try { if (!state.user && !state.token) { redirectToLogin(); return; } if (state.role === "admin") await loadAdmin(); else await loadUser(); await render(); } catch (error) { toast(error.message); } }
function stopPolling() { if (state.polling) { clearInterval(state.polling); state.polling = null; } }
function startPolling(orderID) {
  stopPolling();
  state.polling = setInterval(async () => {
    try {
      const result = await api(`/api/v1/orders/${orderID}`);
      state.currentOrder = result.data;
      const index = state.orders.findIndex(order => order.id === orderID);
      if (index >= 0) state.orders[index] = result.data;
      await render();
      if (["completed", "canceled", "expired_refunded"].includes(result.data.status)) stopPolling();
    } catch (error) { stopPolling(); }
  }, 1000);
}
async function createOrder() { state.busy = true; render(); try { const result = await api("/api/v1/orders", { method: "POST", body: JSON.stringify({ service: selectedService().code, request_id: `web-${Date.now()}` }) }); state.currentOrder = result.data; state.view = "apply"; await refresh(); startPolling(result.data.id); toast(`已分配 ${state.currentOrder.mailbox_address}`); } catch (error) { toast(error.message); } finally { state.busy = false; render(); } }
async function mutateOrder(action) { if (!state.currentOrder) return; try { const result = await api(`/api/v1/orders/${state.currentOrder.id}/${action}`, { method: "POST" }); state.currentOrder = result.data; await refresh(); if (action === "submitted") startPolling(result.data.id); else if (["complete", "cancel"].includes(action)) stopPolling(); toast(action === "submitted" ? "已进入收码等待" : "订单状态已更新"); } catch (error) { toast(error.message); } }
async function selectOrder(id) { const found = state.orders.find(order => order.id === id); if (found) { state.currentOrder = found; state.view = state.role === "admin" ? "admin-orders" : "orders"; await render(); } }

function showSecret(title, secret) {
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal"><div class="card-head"><h2>${esc(title)}</h2><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body"><div class="notice warning">此密钥只显示一次。</div><div class="secret-value"><code>${esc(secret)}</code><button class="ghost-btn" data-action="copy" data-copy="${esc(secret)}">复制</button></div></div></div></div>`);
}

function showServiceEditor(serviceID = "") {
  const service = state.services.find(item => item.id === serviceID) || { id: "", code: "", name: "", description: "", enabled: true, allowed_providers: ["outlook", "hotmail"], price: 0.35, ttl_seconds: 600, sender_domains: [], subject_keywords: [], regex: "\\b(\\d{6})\\b" };
  document.querySelector("#secret-modal")?.remove();
  document.body.insertAdjacentHTML("beforeend", `<div id="secret-modal" class="modal-backdrop"><div class="modal service-modal"><div class="card-head"><h2>${service.id ? "编辑" : "新建"}目标平台</h2><button class="icon-btn" data-action="close-modal" title="关闭">×</button></div><div class="card-body form-grid service-modal-body"><input id="service-id" type="hidden" value="${esc(service.id)}"><div class="form-columns"><label>平台代码<input id="service-code" class="field" value="${esc(service.code)}" placeholder="github"></label><label>平台名称<input id="service-name" class="field" value="${esc(service.name)}"></label></div><label>说明<input id="service-description" class="field" value="${esc(service.description)}"></label><fieldset class="provider-options"><legend>允许邮箱供应商</legend><label><input type="checkbox" name="service-provider" value="outlook" ${(service.allowed_providers || []).includes("outlook") ? "checked" : ""}> Outlook</label><label><input type="checkbox" name="service-provider" value="hotmail" ${(service.allowed_providers || []).includes("hotmail") ? "checked" : ""}> Hotmail</label></fieldset><label>发件人域名<input id="service-senders" class="field" value="${esc((service.sender_domains || []).join(", "))}" placeholder="x.ai"></label><label>主题关键词<input id="service-subjects" class="field" value="${esc((service.subject_keywords || []).join(", "))}" placeholder="verification, 验证码"></label><label>验证码正则<input id="service-regex" class="field" value="${esc(service.regex)}"></label><div class="form-columns"><label>单价<input id="service-price" class="field" type="number" min="0" step="0.01" value="${Number(service.price)}"></label><label>有效期（秒）<input id="service-ttl" class="field" type="number" min="60" max="86400" value="${Number(service.ttl_seconds)}"></label></div></div><div class="service-modal-footer"><label class="check-label"><input id="service-enabled" type="checkbox" ${service.enabled ? "checked" : ""}> 启用该目标平台</label><button class="primary-btn" data-action="save-service">保存平台与收码规则</button></div></div></div>`);
}

const splitList = value => String(value || "").split(",").map(item => item.trim().toLowerCase()).filter(Boolean);
async function saveService() {
  const allowedProviders = [...document.querySelectorAll('input[name="service-provider"]:checked')].map(input => input.value);
  const payload = { id: document.querySelector("#service-id")?.value, code: document.querySelector("#service-code")?.value.trim().toLowerCase(), name: document.querySelector("#service-name")?.value.trim(), description: document.querySelector("#service-description")?.value.trim(), enabled: document.querySelector("#service-enabled")?.checked, allowed_providers: allowedProviders, price: Number(document.querySelector("#service-price")?.value), ttl_seconds: Number(document.querySelector("#service-ttl")?.value), sender_domains: splitList(document.querySelector("#service-senders")?.value), subject_keywords: splitList(document.querySelector("#service-subjects")?.value), regex: document.querySelector("#service-regex")?.value.trim() };
  if (!payload.code || !payload.name || !payload.regex || !payload.allowed_providers.length) return toast("请完整填写平台配置");
  if (payload.allowed_providers.some(provider => !["outlook", "hotmail"].includes(provider))) return toast("邮箱供应商只允许 outlook 和 hotmail");
  if (!payload.sender_domains.length) return toast("至少填写一个发件人域名");
  if (!Number.isFinite(payload.price) || payload.price < 0) return toast("单价不能小于 0");
  if (!Number.isInteger(payload.ttl_seconds) || payload.ttl_seconds < 60 || payload.ttl_seconds > 86400) return toast("有效期必须在 60 到 86400 秒之间");
  await api("/api/v1/admin/services", { method: "POST", body: JSON.stringify(payload) });
  document.querySelector("#secret-modal")?.remove(); await refresh(); toast("目标平台与收码规则已保存");
}

async function deleteService(id) { if (!window.confirm("确定删除该目标平台？已有订单的平台不能删除，可改为停用。")) return; await api(`/api/v1/admin/services/${id}`, { method: "DELETE" }); await refresh(); toast("目标平台已删除"); }
async function deleteMailbox(id) { if (!window.confirm("确定删除该邮箱及其加密凭证？")) return; await api(`/api/v1/admin/mailboxes/${id}`, { method: "DELETE" }); await refresh(); toast("邮箱资源已删除"); }
async function deletePool(id) { if (!window.confirm("确定删除该邮箱池？仅空邮箱池可删除。")) return; await api(`/api/v1/admin/mailbox-pools/${id}`, { method: "DELETE" }); await refresh(); toast("邮箱池已删除"); }

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
async function createPayment() { const amount = Number(document.querySelector("#topup-amount")?.value); const method = document.querySelector("#topup-method")?.value; const result = await api("/api/v1/payment/orders", { method: "POST", body: JSON.stringify({ amount, method, mobile: /Android|iPhone|iPad/i.test(navigator.userAgent) }) }); location.href = result.data.pay_url; }
async function createWebhook() { const url = document.querySelector("#webhook-url")?.value.trim(); const result = await api("/api/v1/webhooks", { method: "POST", body: JSON.stringify({ url }) }); showSecret("Webhook 签名密钥", result.data.secret); await refresh(); }
async function createPool() { const name = document.querySelector("#pool-name")?.value.trim(); if (!name) return toast("请输入邮箱池名称"); await api("/api/v1/admin/mailbox-pools", { method: "POST", body: JSON.stringify({ name, provider: document.querySelector("#pool-provider")?.value, region: document.querySelector("#pool-region")?.value, enabled: true, daily_limit: Number(document.querySelector("#pool-limit")?.value), cooldown_seconds: Number(document.querySelector("#pool-cooldown")?.value) }) }); await refresh(); toast("邮箱池已保存"); }
async function adjustBalance(target) { const amount = Number(window.prompt(`调整 ${target.dataset.email} 的余额，正数充值、负数扣减：`, "10")); if (!amount) return; const description = window.prompt("请输入调整原因：", "管理员人工补偿"); if (!description) return; await api(`/api/v1/admin/users/${target.dataset.id}/balance`, { method: "POST", body: JSON.stringify({ amount, description }) }); await refresh(); toast("余额已调整并写入审计"); }
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
async function startMicrosoftOAuth() { const pool = document.querySelector("#oauth-pool")?.value || state.pools[0]?.name; if (!pool) { state.view = "admin-pools"; await refresh(); return toast("请先创建邮箱池"); } const result = await api("/api/v1/admin/mailboxes/oauth/microsoft", { method: "POST", body: JSON.stringify({ pool }) }); location.href = result.data.authorization_url; }
async function checkUpdates() {
  state.version = (await api("/api/v1/admin/system/version")).data;
  await render();
  const current = String(state.version.current_version || "").replace(/^v/i, "");
  const latest = String(state.version.latest_release?.tag || "").replace(/^v/i, "");
  toast(current && current === latest ? "当前已是最新正式版本" : latest ? `发现新版本 ${state.version.latest_release.tag}` : "暂时无法获取最新版本");
}
async function requestUpgrade() {
  const target = state.version?.latest_release?.tag;
  if (!target) return toast("请先检查更新");
  if (!window.confirm(`确认升级到 ${target}？升级期间服务会短暂重启。`)) return;
  await api("/api/v1/admin/system/upgrade", { method: "POST", headers: { "X-HeroMail-Target-Version": target } });
  toast("升级任务已提交，服务将短暂重启"); startUpgradePolling();
}
function startUpgradePolling() { clearInterval(state.upgradePolling); state.upgradePolling = setInterval(async () => { try { state.version = (await api("/api/v1/admin/system/version")).data; await render(); if (["success", "failed"].includes(state.version.upgrade.state)) clearInterval(state.upgradePolling); } catch (_) {} }, 3000); }

document.addEventListener("click", async event => {
  const target = event.target.closest("[data-action]"); if (!target) return;
  const action = target.dataset.action;
  if (action === "role") { location.href = state.role === "admin" ? "/app" : "/admin"; return; }
  if (action === "toggle-admin-menu") {
    const open = document.body.classList.toggle("mobile-nav-open");
    target.setAttribute("aria-expanded", String(open));
    return;
  }
  if (action === "auth-mode") { location.href = target.dataset.mode === "register" ? "/register" : "/login"; return; }
  if (action === "view") {
    document.body.classList.remove("mobile-nav-open");
    document.querySelector(".mobile-admin-menu")?.setAttribute("aria-expanded", "false");
    await navigate(target.dataset.view); return;
  }
  if (action === "service") { state.selectedService = target.dataset.service; await render(); return; }
  if (action === "create") { await createOrder(); return; }
  if (action === "submit") { state.currentOrder = state.orders.find(order => order.id === target.dataset.order) || state.currentOrder; await mutateOrder("submitted"); return; }
  if (action === "complete") { await mutateOrder("complete"); return; }
  if (action === "cancel") { await mutateOrder("cancel"); return; }
  if (action === "select-order") { await selectOrder(target.dataset.order); return; }
  if (action === "refresh") { await refresh(); return; }
  if (action === "copy") { await navigator.clipboard?.writeText(target.dataset.copy || ""); toast("已复制"); return; }
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
  if (action === "create-pool") { await createPool(); return; }
  if (action === "view-pool") {
    state.mailboxPool = target.dataset.pool || "";
    state.pagination.mailboxes = { page: 1 };
    history.pushState({}, "", `/admin/mailboxes?pool=${encodeURIComponent(state.mailboxPool)}`);
    state.view = "admin-mailboxes";
    await refresh(); return;
  }
  if (action === "import-mailboxes") { await importMailboxes(); return; }
  if (action === "delete-pool") { await deletePool(target.dataset.id); return; }
  if (action === "delete-mailbox") { await deleteMailbox(target.dataset.id); return; }
  if (action === "verify-mailbox") { await verifyMailbox(target.dataset.id); return; }
  if (action === "edit-service") { showServiceEditor(target.dataset.id || ""); return; }
  if (action === "save-service") { await saveService(); return; }
  if (action === "delete-service") { await deleteService(target.dataset.id); return; }
  if (action === "adjust-balance") { await adjustBalance(target); return; }
  if (action === "edit-payment-provider") { await showPaymentProviderEditor(target.dataset.id || ""); return; }
  if (action === "save-payment-provider") { await savePaymentProvider(); return; }
  if (action === "delete-payment-provider") { await deletePaymentProvider(target); return; }
  if (action === "microsoft-oauth") { await startMicrosoftOAuth(); return; }
  if (action === "check-updates") { await checkUpdates(); return; }
  if (action === "upgrade") { await requestUpgrade(); return; }
  if (action === "close-modal") { document.querySelector("#secret-modal")?.remove(); return; }
});

window.addEventListener("popstate", async () => {
  if (!state.user) return;
  state.view = routeView(state.role);
  state.mailboxPool = new URLSearchParams(location.search).get("pool") || "";
  await refresh();
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
