const publicState = { token: localStorage.getItem("heromail_token") || "", user: null };
const publicContent = document.querySelector("#public-content");
const escPublic = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[char]));
const publicMoney = value => `¥${Number(value || 0).toFixed(2)}`;

function publicToast(message) {
  const element = document.querySelector("#public-toast");
  element.textContent = message;
  element.classList.add("show");
  clearTimeout(publicToast.timer);
  publicToast.timer = setTimeout(() => element.classList.remove("show"), 2800);
}

async function publicAPI(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  if (publicState.token) headers.Authorization = `Bearer ${publicState.token}`;
  const response = await fetch(path, { ...options, headers });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.message || "请求失败");
  return body;
}

function pageShell(kicker, title, description, body) {
  return `<section class="public-page"><div class="public-page-inner"><div class="public-page-head"><span class="hero-kicker">${kicker}</span><h1>${title}</h1><p>${description}</p></div>${body}</div></section>`;
}

async function renderPricing() {
  publicContent.innerHTML = pageShell("公开价格", "按目标平台清晰计费", "创建订单时预扣余额，分配失败立即释放；超时未收到验证码自动退款，成功收码后完成结算。", `<div class="public-empty">正在加载价格…</div>`);
  try {
    const result = await publicAPI("/api/v1/public/services?page=1&page_size=100");
    const cards = result.data.map(service => `<article class="price-item"><span class="price-code">${escPublic(service.code)}</span><h2>${escPublic(service.name)}</h2><p>${escPublic(service.description)} · ${Math.max(1, Math.round(service.ttl_seconds / 60))} 分钟有效 · ${escPublic(service.allowed_providers.join(" / "))}</p><strong>${publicMoney(service.price)}</strong><small>每次成功收码任务</small></article>`).join("");
    publicContent.innerHTML = pageShell("公开价格", "按目标平台清晰计费", "创建订单时预扣余额，分配失败立即释放；超时未收到验证码自动退款，成功收码后完成结算。", `<div class="pricing-grid">${cards || `<div class="public-empty">暂未启用可申请平台</div>`}</div>`);
  } catch (error) {
    publicToast(error.message);
  }
}

function renderDocs() {
  const createExample = `POST /api/v1/orders\nContent-Type: application/json\n\n{\n  "service": "github",\n  "request_id": "client-request-001"\n}`;
  publicContent.innerHTML = pageShell("开发者文档", "从 API 创建注册收码任务", "文档公开可读。API Key、Webhook 配置和调用记录需要登录后在用户门户管理。", `<div class="docs-layout"><nav class="docs-nav"><a href="#concept">核心概念</a><a href="#auth">鉴权</a><a href="#create">创建订单</a><a href="#status-api">查询状态</a><a href="#webhook">Webhook</a><a href="#errors">错误处理</a></nav><div class="docs-content"><section id="concept"><h2>核心概念</h2><p>用户提交目标平台代码，HeroMail 从允许的 Outlook / Hotmail 邮箱池中原子分配资源。返回结果只包含本次任务所需的邮箱地址、状态、过期时间和验证码。</p></section><section id="auth"><h2>鉴权</h2><p>网页使用 Bearer 会话，服务端集成使用只展示一次的 <code>hm_</code> API Key。密钥按范围授权并只保存哈希。</p><pre class="public-code">Authorization: Bearer hm_your_api_key</pre></section><section id="create"><h2>创建订单</h2><pre class="public-code">${escPublic(createExample)}</pre><p><code>request_id</code> 用于幂等创建。同一用户重复提交相同值不会产生重复扣款。</p></section><section id="status-api"><h2>查询订单</h2><pre class="public-code">GET /api/v1/orders/{id}</pre><p>状态依次为 <code>assigned</code>、<code>waiting_code</code>、<code>code_received</code> 和 <code>completed</code>。所有列表使用 <code>page</code> 与 <code>page_size</code> 服务端分页。</p></section><section id="webhook"><h2>Webhook</h2><p>订单状态变化可使用 HMAC 签名 Webhook 推送。投递失败会指数退避，用户可在门户查看记录并手动重试。</p></section><section id="errors"><h2>错误处理</h2><p>接口错误统一返回 <code>error</code> 与 <code>message</code>。分配失败不会扣款；等待验证码超时会自动退款并生成资金流水。</p></section></div></div>`);
}

async function renderStatus() {
  publicContent.innerHTML = pageShell("服务状态", "HeroMail 当前运行状态", "公开状态仅展示整体可用性，不公开邮箱库存、渠道凭证或内部规则。", `<div class="public-empty">正在检查服务…</div>`);
  try {
    const result = (await publicAPI("/api/v1/public/status")).data;
    const rows = [["HTTP API", result.api], ["邮箱分配服务", result.allocation], ["Microsoft Graph 收码", result.mail_worker]];
    publicContent.innerHTML = pageShell("服务状态", result.status === "operational" ? "所有公开服务运行正常" : "部分服务性能下降", `最后检查时间：${new Date(result.updated_at).toLocaleString("zh-CN")}`, `<div class="status-panel">${rows.map(([name, status]) => `<div class="status-row"><div><i class="status-indicator"></i><strong>${name}</strong></div><span class="status-label">${status === "operational" ? "运行正常" : "性能下降"}</span></div>`).join("")}</div>`);
  } catch (error) {
    publicToast(error.message);
  }
}

function renderOpenSource() {
  publicContent.innerHTML = pageShell("开源与自托管", "数据、邮箱资产和升级流程由你掌控", "HeroMail 使用 MIT 许可证发布，正式镜像、安装脚本和版本日志均来自 GitHub Release。", `<div class="docs-content"><section><h2>一行命令安装</h2><pre class="public-code">curl -fsSL https://github.com/ljunn/heromail/releases/latest/download/install.sh | sudo bash</pre></section><section><h2>默认技术栈</h2><p>Go + Gin 提供 API 和内嵌前端，PostgreSQL 保存业务事实，Redis 负责跨进程锁与协调，Microsoft Graph Worker 拉取并匹配验证码邮件。</p></section><section><h2>正式升级</h2><p>首次安装绑定 GitHub 正式 Release。后续版本在管理后台检查更新并点击在线升级，升级器只允许官方 <code>ghcr.io/ljunn/heromail:latest</code> 镜像。</p><a class="solid-action" href="https://github.com/ljunn/heromail" target="_blank" rel="noopener">查看 GitHub 仓库</a></section></div>`);
}

function renderAuth(register) {
  const title = register ? "创建 HeroMail 账户" : "登录 HeroMail";
  publicContent.innerHTML = `<section class="auth-page"><div class="auth-box"><a class="public-brand" href="/"><img src="/brand-mark.svg" alt=""><strong>HeroMail</strong></a><h1>${title}</h1><p>${register ? "注册后即可选择目标平台、申请邮箱并配置 API 能力。" : "进入用户门户申请邮箱、查看任务和管理余额。"}</p><form id="public-auth-form" class="auth-form"><label>邮箱<input name="email" type="email" autocomplete="email" required></label>${register ? `<label>显示名称<input name="display_name" autocomplete="name"></label>` : ""}<label>密码<input name="password" type="password" minlength="10" autocomplete="${register ? "new-password" : "current-password"}" required></label><button type="submit">${register ? "注册并进入门户" : "登录"}</button></form><div class="auth-switch">${register ? `已有账户？<a href="/login">返回登录</a>` : `还没有账户？<a href="/register">创建账户</a>`}</div></div></section>`;
  document.querySelector("#public-auth-form").addEventListener("submit", event => submitAuth(event, register));
}

async function submitAuth(event, register) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  try {
    const result = await publicAPI(`/api/v1/auth/${register ? "register" : "login"}`, { method: "POST", body: JSON.stringify({ email: form.get("email"), password: form.get("password"), display_name: form.get("display_name") || "" }) });
    publicState.token = result.data.token;
    publicState.user = result.data.user;
    localStorage.setItem("heromail_token", result.data.token);
    const requested = new URLSearchParams(location.search).get("redirect") || "";
    const allowedRedirect = requested.startsWith("/app") || (result.data.user.role === "admin" && requested.startsWith("/admin"));
    location.href = allowedRedirect ? requested : result.data.user.role === "admin" ? "/admin" : "/app";
  } catch (error) {
    publicToast(error.message);
  }
}

async function resolvePublicSession() {
  if (!publicState.token) return;
  try {
    publicState.user = await publicAPI("/api/v1/me");
    const target = publicState.user.role === "admin" ? "/admin" : "/app";
    document.querySelector("#public-actions").innerHTML = `<a class="text-action" href="${target}">欢迎，${escPublic(publicState.user.display_name || publicState.user.email)}</a><a class="solid-action" href="${target}">进入工作台</a>`;
  } catch (_) {
    publicState.token = "";
    localStorage.removeItem("heromail_token");
  }
}

function initHomeAnimations() {
  if (!window.gsap) return;
  const { gsap } = window;
  const ScrollTrigger = window.ScrollTrigger;
  if (ScrollTrigger) gsap.registerPlugin(ScrollTrigger);

  const mm = gsap.matchMedia();
  mm.add({
    desktop: "(min-width: 761px)",
    reduced: "(prefers-reduced-motion: reduce)"
  }, context => {
    const { desktop, reduced } = context.conditions;
    const hero = document.querySelector(".public-hero");
    const stage = document.querySelector(".hero-stage");
    let cleanupPointer = () => {};
    if (!hero || !stage) return undefined;

    if (reduced) {
      gsap.set([".hero-kicker", ".hero-copy h1 > span", ".hero-copy h1 strong", ".hero-copy p", ".hero-actions", ".hero-metrics", ".stage-console", ".stage-float", ".stage-label", ".signal-strip > div", "[data-reveal]"], { autoAlpha: 1, clearProps: "transform" });
      return undefined;
    }

    const intro = gsap.timeline({ defaults: { duration: .65, ease: "power3.out" } });
    intro.from(".hero-kicker", { autoAlpha: 0, y: 16 })
      .from(".hero-copy h1 > span", { autoAlpha: 0, y: 28 }, "-=.35")
      .from(".hero-copy h1 strong", { autoAlpha: 0, y: 22 }, "-=.35")
      .from(".hero-copy p", { autoAlpha: 0, y: 18 }, "-=.35")
      .from(".hero-actions", { autoAlpha: 0, y: 16 }, "-=.32")
      .from(".hero-metrics > div", { autoAlpha: 0, y: 14, stagger: .1 }, "-=.28")
      .from(".stage-console", { autoAlpha: 0, y: 32, rotation: 5, duration: .9 }, "-=.68")
      .from(".stage-float, .stage-label", { autoAlpha: 0, y: 18, stagger: .12, duration: .45 }, "-=.45");

    gsap.to(".stage-orbit", { rotation: "+=360", duration: 34, ease: "none", repeat: -1 });
    gsap.to(".stage-float", { y: "-=8", duration: 2.4, ease: "sine.inOut", repeat: -1, yoyo: true, stagger: .32 });
    gsap.to(".feed-pulse", { scale: 1.45, autoAlpha: .42, duration: .8, ease: "sine.inOut", repeat: -1, yoyo: true, stagger: .2 });

    const code = document.querySelector(".code-digits");
    if (code) intro.call(() => { code.textContent = "482 916"; }, null, "+=.15");

    if (desktop) {
      const xTo = gsap.quickTo(stage, "x", { duration: .8, ease: "power3.out" });
      const yTo = gsap.quickTo(stage, "y", { duration: .8, ease: "power3.out" });
      const moveStage = event => {
        const bounds = stage.getBoundingClientRect();
        xTo((event.clientX - bounds.left - bounds.width / 2) * .018);
        yTo((event.clientY - bounds.top - bounds.height / 2) * .012);
      };
      const resetStage = () => { xTo(0); yTo(0); };
      stage.addEventListener("pointermove", moveStage);
      stage.addEventListener("pointerleave", resetStage);
      cleanupPointer = () => {
        stage.removeEventListener("pointermove", moveStage);
        stage.removeEventListener("pointerleave", resetStage);
      };
    }

    if (!ScrollTrigger) return cleanupPointer;
    gsap.from(".signal-strip > div", {
      autoAlpha: 0,
      y: 20,
      stagger: .1,
      duration: .55,
      ease: "power2.out",
      scrollTrigger: { trigger: ".signal-strip", start: "top 86%", once: true }
    });
    gsap.utils.toArray("[data-reveal]").forEach(element => {
      gsap.from(element, {
        autoAlpha: 0,
        y: 32,
        duration: .7,
        ease: "power3.out",
        scrollTrigger: { trigger: element, start: "top 84%", once: true }
      });
    });
    gsap.to(stage, {
      yPercent: -4,
      ease: "none",
      scrollTrigger: { trigger: hero, start: "top top", end: "bottom top", scrub: 1 }
    });
    window.addEventListener("load", () => ScrollTrigger.refresh(), { once: true });
    ScrollTrigger.refresh();
    return cleanupPointer;
  });
}

document.addEventListener("click", event => {
  const action = event.target.closest("[data-public-action]");
  if (!action) return;
  if (action.dataset.publicAction === "menu") {
    const nav = document.querySelector("#public-nav");
    nav.classList.toggle("open");
    action.setAttribute("aria-expanded", String(nav.classList.contains("open")));
  }
});

const path = location.pathname;
if (path === "/pricing") renderPricing();
else if (path === "/docs" || path.startsWith("/docs/")) renderDocs();
else if (path === "/status") renderStatus();
else if (path === "/open-source") renderOpenSource();
else if (path === "/login") renderAuth(false);
else if (path === "/register") renderAuth(true);
else if (path === "/") initHomeAnimations();
resolvePublicSession();
