const state = {
  role: "user",
  view: "apply",
  services: [],
  orders: [],
  mailboxes: [],
  overview: null,
  selectedService: "github",
  currentOrder: null,
  polling: null
};

const userNav = [
  ["工作台", [["apply", "申请邮箱", "✉"], ["current", "当前任务", "◷"], ["orders", "订单记录", "▤"]]],
  ["开发者", [["docs", "API 文档", "▥"], ["keys", "API 密钥", "⌕"]]],
  ["账户", [["usage", "用量与账单", "▥"], ["balance", "余额充值", "▣"], ["settings", "个人设置", "⚙"], ["status", "服务状态", "♢"]]]
];

const adminNav = [
  ["运行", [["admin-overview", "运行概览", "▣"]]],
  ["资源管理", [["admin-mailboxes", "邮箱资源", "✉"], ["admin-pools", "邮箱池", "▤"], ["admin-channels", "接入渠道", "⌁"]]],
  ["业务配置", [["admin-services", "目标平台", "◈"], ["admin-rules", "收码规则", "⌘"], ["admin-routing", "调度策略", "◇"]]],
  ["订单与用户", [["admin-orders", "注册订单", "▤"], ["admin-users", "平台用户", "♙"], ["admin-ledger", "余额与流水", "▣"]]],
  ["系统", [["admin-alerts", "告警中心", "♢"], ["admin-audit", "审计日志", "▤"], ["admin-settings", "系统设置", "⚙"]]]
];

async function api(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  headers["X-HeroMail-Role"] = state.role;
  headers["X-HeroMail-User"] = state.role === "admin" ? "admin-001" : "user-001";
  const response = await fetch(path, { ...options, headers });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.message || "请求失败");
  return body;
}

const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[char]));
const money = value => `¥${Number(value || 0).toFixed(2)}`;
const time = value => value ? new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "-";
const statusMap = { assigned: ["等待提交", "orange"], waiting_code: ["收码中", "blue"], code_received: ["已收码", "green"], completed: ["已完成", "green"], canceled: ["已取消", "red"], expired_refunded: ["已超时退款", "orange"], allocation_failed: ["分配失败", "red"] };
const statusChip = status => { const item = statusMap[status] || [status, "blue"]; return `<span class="chip ${item[1]}">${item[0]}</span>`; };

function toast(message) {
  const el = document.querySelector("#toast"); el.textContent = message; el.classList.add("show");
  clearTimeout(toast.timer); toast.timer = setTimeout(() => el.classList.remove("show"), 2600);
}

function renderNav() {
  const groups = state.role === "admin" ? adminNav : userNav;
  document.querySelector("#role-label").textContent = state.role === "admin" ? "管理员端" : "用户端";
  document.querySelector("#role-action").textContent = state.role === "admin" ? "切换用户端" : "切换管理员端";
  document.querySelector("#nav").innerHTML = groups.map(([title, items]) => `<div class="nav-group"><div class="nav-title">${title}</div>${items.map(([view, label, icon]) => `<button class="nav-item ${state.view === view ? "active" : ""}" data-action="view" data-view="${view}"><span class="nav-icon">${icon}</span>${label}</button>`).join("")}</div>`).join("");
}

function stat(label, value, note = "") { return `<div class="stat"><div class="stat-label">${label}</div><div class="stat-value">${value}</div><div class="stat-note">${note}</div></div>`; }
function pageHead(title, subtitle, action = "") { return `<div class="page-head"><div><h1>${title}</h1><p>${subtitle}</p></div>${action ? `<div class="head-actions">${action}</div>` : ""}</div>`; }

function serviceCard(service) {
  const selected = service.code === state.selectedService;
  return `<button class="service-card ${selected ? "selected" : ""}" data-action="service" data-service="${esc(service.code)}"><span class="service-logo">${esc(service.name.slice(0, 1))}</span><span class="service-name">${esc(service.name)}</span><span class="service-desc">${esc(service.description)}</span>${selected ? `<span class="selected-mark">✓</span>` : ""}</button>`;
}

function selectedService() { return state.services.find(service => service.code === state.selectedService) || state.services[0] || { name: "目标平台", price: 0.35, ttl_seconds: 600, allowed_providers: ["outlook", "hotmail"] }; }

function renderApply() {
  const service = selectedService();
  const current = state.currentOrder;
  return pageHead("申请邮箱", "选择目标平台，系统自动分配临时邮箱并提取注册验证码。", `<button class="ghost-btn" data-action="view" data-view="orders">查看订单记录</button>`) + `<div class="card" style="margin-bottom:16px"><div class="card-head"><h2>1. 选择目标平台</h2><span class="muted">平台邮箱由系统自动分配</span></div><div class="card-body"><div class="service-grid">${state.services.map(serviceCard).join("")}</div></div></div><div class="work-grid"><div class="card"><div class="card-head"><h2>2. 订单配置</h2></div><div class="card-body"><dl class="config-list"><div class="config-row"><dt>目标平台</dt><dd>${esc(service.name)}</dd></div><div class="config-row"><dt>邮箱类型</dt><dd>自动分配（Outlook / Hotmail）</dd></div><div class="config-row"><dt>可用库存</dt><dd>1,248 个</dd></div><div class="config-row"><dt>预计收码</dt><dd>30–60 秒</dd></div><div class="config-row"><dt>有效期</dt><dd>10 分钟</dd></div><div class="config-row"><dt>单价</dt><dd>${money(service.price)} / 次</dd></div></dl><button class="primary-btn" style="width:100%;margin-top:18px" data-action="create" ${state.busy ? "disabled" : ""}>${state.busy ? "正在分配…" : "申请邮箱"}</button></div></div><div class="card task-card"><div class="card-head"><h2>3. 当前任务</h2>${current ? statusChip(current.status) : ""}</div><div class="card-body">${current ? renderTask(current) : `<div class="empty">提交申请后，已分配邮箱和验证码会显示在这里。</div>`}</div></div><div class="card"><div class="card-head"><h2>服务状态</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> Outlook 服务 <span class="status-ok">正常</span></div><div class="status-item"><i class="status-dot"></i> Hotmail 服务 <span class="status-ok">正常</span></div><div class="status-item"><i class="status-dot"></i> 验证码提取 <span class="status-ok">正常</span></div><div class="status-item"><i class="status-dot"></i> API 接口 <span class="status-ok">正常</span></div></div><div class="notice">邮箱仅用于本次平台注册，凭证不会提供。有效期内未收到匹配验证码会自动退款。</div></div></div></div>${renderRecentOrders()}`;
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
  return pageHead("运行概览", "邮箱库存、平台注册任务与收码服务的实时状态。", `<button class="ghost-btn" data-action="refresh">刷新数据</button>`) + `<div class="stat-grid">${stat("可分配邮箱", data.available_mailboxes ?? 0, "全局健康可用")}${stat("活跃租约", data.active_leases ?? 0, "10 分钟任务")}${stat("今日注册订单", data.today_orders ?? 0, "含分配失败")}${stat("收码成功率", `${(data.success_rate || 98.65).toFixed(2)}%`, "较昨日 +0.32%")}${stat("平均收码时间", `${(data.average_code_seconds || 23.5).toFixed(1)} 秒`, "较昨日 -1.2 秒")}</div><div class="admin-grid"><div class="card"><div class="card-head"><h2>目标平台库存</h2><button class="link-btn" data-action="view" data-view="admin-services">管理平台 →</button></div><div class="card-body"><div class="bar-list">${services.map((service, i) => `<div class="bar-row"><span>${esc(service.name)}</span><div class="bar-track"><div class="bar-fill" style="width:${88 - i * 8}%"></div></div><strong>${Math.max(82, 99 - i * 3)}%</strong></div>`).join("")}</div></div></div><div class="card"><div class="card-head"><h2>邮箱渠道健康</h2></div><div class="card-body"><div class="status-list"><div class="status-item"><i class="status-dot"></i> Outlook <span class="status-ok">授权有效</span></div><div class="status-item"><i class="status-dot"></i> Hotmail <span class="status-ok">授权有效</span></div><div class="status-item"><i class="status-dot"></i> 验证码 Worker <span class="status-ok">运行中</span></div></div></div></div></div><div class="admin-grid"><div class="card"><div class="card-head"><h2>最近注册订单</h2><button class="link-btn" data-action="view" data-view="admin-orders">查看全部 →</button></div><div class="table-wrap">${orders.length ? `<table><thead><tr><th>订单号</th><th>用户</th><th>平台</th><th>邮箱</th><th>状态</th><th>剩余</th></tr></thead><tbody>${orders.map(order => `<tr><td>${esc(order.id)}</td><td>${esc(order.user_id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${order.status === "completed" ? "—" : time(order.expires_at)}</td></tr>`).join("")}</tbody></table>` : `<div class="empty">暂无订单</div>`}</div></div><div class="card"><div class="card-head"><h2>快捷操作</h2></div><div class="card-body"><div class="service-grid"><button class="service-card" data-action="view" data-view="admin-mailboxes"><span class="service-logo">✉</span><span class="service-name">邮箱资源</span><span class="service-desc">检查授权与库存</span></button><button class="service-card" data-action="view" data-view="admin-services"><span class="service-logo">◈</span><span class="service-name">目标平台</span><span class="service-desc">配置规则和价格</span></button></div></div></div></div>`;
}

function renderAdminMailboxes() {
  return pageHead("邮箱资源", "邮箱是平台资产，系统按“邮箱 × 目标平台”决定是否分配。", `<button class="primary-btn" data-action="message" data-message="批量导入将在接入 Microsoft OAuth 后启用">添加邮箱</button>`) + `<div class="stat-grid">${stat("邮箱总数", state.mailboxes.length, "已接入资产")}${stat("全局可用", state.mailboxes.filter(mailbox => mailbox.state === "available").length, "健康分 ≥ 60")}${stat("租用中", state.mailboxes.filter(mailbox => mailbox.state === "leased").length, "注册任务")}${stat("授权异常", state.mailboxes.filter(mailbox => mailbox.state === "auth_error").length, "需要重新授权")}${stat("隔离", state.mailboxes.filter(mailbox => mailbox.state === "blocked").length, "人工处理")}</div><div class="card"><div class="filter-bar"><select class="select"><option>全部供应商</option><option>Microsoft</option></select><select class="select"><option>全部邮箱池</option><option>Outlook Pool A</option><option>Hotmail Pool A</option></select><select class="select"><option>全部状态</option><option>可用</option><option>租用中</option></select><input class="search" placeholder="搜索邮箱地址"></div><div class="table-wrap"><table><thead><tr><th>邮箱</th><th>供应商</th><th>邮箱池</th><th>授权状态</th><th>健康分</th><th>活跃订单</th><th>今日收码</th><th>最近收信</th><th>状态</th></tr></thead><tbody>${state.mailboxes.map(mailbox => `<tr><td>${esc(mailbox.address)}</td><td>Microsoft</td><td>${esc(mailbox.pool)}</td><td><span class="chip green">有效</span></td><td>${mailbox.health_score}/100</td><td>${esc(mailbox.active_order_id || "—")}</td><td>${mailbox.today_codes}</td><td>${time(mailbox.last_received_at)}</td><td>${mailbox.state === "available" ? `<span class="chip green">可用</span>` : `<span class="chip orange">${esc(mailbox.state)}</span>`}</td></tr>`).join("")}</tbody></table></div></div>`;
}

function renderAdminServices() {
  return pageHead("目标平台", "配置允许使用的邮箱池、发件人规则、验证码解析和订单结算。", `<button class="primary-btn" data-action="message" data-message="新建平台配置将在下一阶段开放">新建目标平台</button>`) + `<div class="card"><div class="table-wrap"><table><thead><tr><th>平台</th><th>平台代码</th><th>状态</th><th>可分配邮箱</th><th>租用中</th><th>已消费</th><th>单价</th><th>有效期</th><th>操作</th></tr></thead><tbody>${state.services.map(service => `<tr><td><strong>${esc(service.name)}</strong><div class="muted">${esc(service.description)}</div></td><td><code>${esc(service.code)}</code></td><td><span class="chip green">启用</span></td><td>—</td><td>—</td><td>—</td><td>${money(service.price)}</td><td>${Math.round(service.ttl_seconds / 60)} 分钟</td><td><button class="link-btn" data-action="message" data-message="平台规则编辑器将在下一阶段开放">查看规则</button></td></tr>`).join("")}</tbody></table></div></div><div class="card" style="margin-top:16px"><div class="card-head"><h2>统一业务规则</h2></div><div class="card-body"><div class="notice">邮箱仅用于一次平台注册；同一个邮箱在同一平台收到验证码后标记为已消费。任务有效期为 10 分钟，收到验证码即结算，超时未收码自动退款。</div></div></div>`;
}

function renderAdminOrders() {
  return pageHead("注册订单", "监控平台注册全链路、收码状态、退款和邮箱平台占用。", `<button class="ghost-btn" data-action="refresh">刷新数据</button>`) + `<div class="stat-grid">${stat("今日订单", state.orders.length, "演示数据")}${stat("已收码", state.orders.filter(order => order.code).length, "验证码已匹配")}${stat("收码中", state.orders.filter(order => order.status === "waiting_code").length, "等待邮件")}${stat("已退款", state.orders.filter(order => order.refunded).length, "超时或取消")}${stat("异常订单", 0, "需要人工处理")}</div><div class="card"><div class="filter-bar"><select class="select"><option>全部平台</option></select><select class="select"><option>全部状态</option></select><input class="search" placeholder="搜索订单号、邮箱或用户"></div><div class="table-wrap"><table><thead><tr><th>订单号</th><th>用户</th><th>目标平台</th><th>分配邮箱</th><th>状态</th><th>验证码</th><th>费用</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${state.orders.map(order => `<tr class="${state.currentOrder && state.currentOrder.id === order.id ? "selected" : ""}"><td>${esc(order.id)}</td><td>${esc(order.user_id)}</td><td>${esc(order.service_name)}</td><td>${esc(order.mailbox_address)}</td><td>${statusChip(order.status)}</td><td>${esc(order.code || "—")}</td><td>${money(order.price)}</td><td>${time(order.created_at)}</td><td><button class="link-btn" data-action="select-order" data-order="${order.id}">详情</button></td></tr>`).join("")}</tbody></table></div></div>${state.currentOrder ? `<div class="card" style="margin-top:16px"><div class="card-head"><h2>订单详情 · ${esc(state.currentOrder.id)}</h2><span>${statusChip(state.currentOrder.status)}</span></div><div class="card-body drawer-grid"><div class="timeline">${[["创建订单", state.currentOrder.created_at], ["分配邮箱", state.currentOrder.assigned_at], ["用户已提交", state.currentOrder.submitted_at], ["收到验证码", state.currentOrder.code_received_at], ["完成结算", state.currentOrder.completed_at]].map(([label, value]) => `<div class="timeline-item ${value ? "done" : ""}"><span class="timeline-dot"></span><div><div class="timeline-title">${label}</div><div class="timeline-time">${time(value)}</div></div></div>`).join("")}</div><div><dl class="config-list"><div class="config-row"><dt>目标平台</dt><dd>${esc(state.currentOrder.service_name)}</dd></div><div class="config-row"><dt>分配邮箱</dt><dd>${esc(state.currentOrder.mailbox_address)}</dd></div><div class="config-row"><dt>验证码</dt><dd>${esc(state.currentOrder.code || "—")}</dd></div><div class="config-row"><dt>费用</dt><dd>${money(state.currentOrder.price)}</dd></div></dl></div></div></div>` : ""}`;
}

function renderPlaceholder(title) { return pageHead(title, "该模块已预留 API 和导航位置。", `<button class="ghost-btn" data-action="view" data-view="${state.role === "admin" ? "admin-overview" : "apply"}">返回工作台</button>`) + `<div class="card"><div class="empty">${title}正在接入中，当前版本先保证邮箱分配、验证码订单和资源监控闭环。</div></div>`; }

function render() {
  renderNav();
  const views = { apply: renderApply, current: renderCurrent, orders: renderOrders, "admin-overview": renderAdminOverview, "admin-mailboxes": renderAdminMailboxes, "admin-services": renderAdminServices, "admin-orders": renderAdminOrders, docs: () => renderPlaceholder("API 文档"), keys: () => renderPlaceholder("API 密钥"), usage: () => renderPlaceholder("用量与账单"), balance: () => renderPlaceholder("余额充值"), settings: () => renderPlaceholder("个人设置"), status: () => renderPlaceholder("服务状态") };
  document.querySelector("#content").innerHTML = (views[state.view] || views.apply)();
  const mePromise = api("/api/v1/me").then(user => { document.querySelector("#balance").textContent = `余额 ${money(user.balance)}`; }).catch(() => {});
  return mePromise;
}

async function loadUser() { const [services, orders] = await Promise.all([api("/api/v1/services"), api("/api/v1/orders")]); state.services = services.data || []; state.orders = orders.data || []; if (!state.selectedService && state.services[0]) state.selectedService = state.services[0].code; }
async function loadAdmin() { const [overview, services, mailboxes, orders] = await Promise.all([api("/api/v1/admin/overview"), api("/api/v1/admin/services"), api("/api/v1/admin/mailboxes"), api("/api/v1/admin/orders")]); state.overview = overview.data; state.services = services.data || []; state.mailboxes = mailboxes.data || []; state.orders = orders.data || []; }

async function refresh() { try { if (state.role === "admin") await loadAdmin(); else await loadUser(); await render(); } catch (error) { toast(error.message); } }
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

document.addEventListener("click", async event => {
  const target = event.target.closest("[data-action]"); if (!target) return;
  const action = target.dataset.action;
  if (action === "role") { stopPolling(); state.role = state.role === "user" ? "admin" : "user"; state.view = state.role === "admin" ? "admin-overview" : "apply"; state.currentOrder = null; await refresh(); return; }
  if (action === "view") { state.view = target.dataset.view; await refresh(); return; }
  if (action === "service") { state.selectedService = target.dataset.service; await render(); return; }
  if (action === "create") { await createOrder(); return; }
  if (action === "submit") { state.currentOrder = state.orders.find(order => order.id === target.dataset.order) || state.currentOrder; await mutateOrder("submitted"); return; }
  if (action === "complete") { await mutateOrder("complete"); return; }
  if (action === "cancel") { await mutateOrder("cancel"); return; }
  if (action === "select-order") { await selectOrder(target.dataset.order); return; }
  if (action === "refresh") { await refresh(); return; }
  if (action === "copy") { await navigator.clipboard?.writeText(target.dataset.copy || ""); toast("已复制"); return; }
  if (action === "message") { toast(target.dataset.message || "该功能正在接入中"); }
});

async function boot() { try { await loadUser(); await render(); } catch (error) { toast(error.message); } }
boot();
