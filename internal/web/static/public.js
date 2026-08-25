const publicState = { token: localStorage.getItem("heromail_token") || "", user: null };
const publicContent = document.querySelector("#public-content");
const escPublic = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[char]));
const publicMoney = value => `¥${Number(value || 0).toFixed(2)}`;
const publicProviderLabels = { outlook: "Outlook", outlook_de: "Outlook.de", hotmail: "Hotmail", gmail: "Gmail", icloud: "iCloud", mailcom: "Mail.com" };

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
  const pricingHead = ["公开价格", "先选平台，再选邮箱类型", "不同邮箱渠道独立定价。下单时可以多选可接受类型，系统只按最终分配成功的邮箱类型结算。"];
  const flow = `<div class="pricing-flow" aria-label="下单与计费流程"><div class="pricing-flow-item"><span>01</span><div><strong>选择目标平台</strong><small>例如 OpenAI、Grok</small></div></div><i aria-hidden="true">→</i><div class="pricing-flow-item"><span>02</span><div><strong>勾选邮箱类型</strong><small>可多选，增加分配成功率</small></div></div><i aria-hidden="true">→</i><div class="pricing-flow-item"><span>03</span><div><strong>按实际类型结算</strong><small>超时未收码自动退款</small></div></div></div>`;
  publicContent.innerHTML = pageShell(pricingHead[0], pricingHead[1], pricingHead[2], `${flow}<div class="public-empty">正在加载价格…</div>`);
  try {
    const result = await publicAPI("/api/v1/public/services?page=1&page_size=100");
    const cards = (result.data || []).map(service => {
      const providers = service.allowed_providers || [];
      const prices = providers.map(provider => `<div class="public-price-row"><span>${escPublic(publicProviderLabels[provider] || provider)}</span><strong>${publicMoney(service.provider_prices?.[provider])}<small>/ 次</small></strong></div>`).join("");
      const ttl = Math.max(1, Math.round(service.ttl_seconds / 60));
      return `<article class="price-item"><div class="price-item-top"><span class="price-code">${escPublic(service.code)}</span><span class="price-item-badge">${providers.length} 种可选</span></div><h2>${escPublic(service.name)}</h2><p>${escPublic(service.description || "注册验证码收取服务")}</p><div class="price-item-rule"><span>订单有效期</span><strong>${ttl} 分钟</strong></div><div class="public-provider-prices"><div class="provider-price-heading"><span>邮箱类型</span><span>单次价格</span></div>${prices || `<div class="public-empty compact">暂未配置邮箱类型</div>`}</div><div class="price-item-foot"><span>创建订单即预扣</span><span>未收码自动退款</span></div></article>`;
    }).join("");
    publicContent.innerHTML = pageShell(pricingHead[0], pricingHead[1], pricingHead[2], `${flow}<div class="pricing-grid">${cards || `<div class="public-empty">暂未启用可申请平台</div>`}</div>`);
  } catch (error) {
    publicToast(error.message);
  }
}

function renderDocs() {
  const serviceExample = `GET /api/v1/services?page=1&page_size=20\nAuthorization: Bearer hm_your_api_key`;
  const createExample = `POST /api/v1/orders\nAuthorization: Bearer hm_your_api_key\nContent-Type: application/json\n\n{\n  "service": "openai",\n  "mailbox_providers": ["outlook", "hotmail"],\n  "request_id": "client-request-001"\n}`;
  const responseExample = `{\n  "data": {\n    "id": "ord_01J...",\n    "service_name": "OpenAI",\n    "mailbox_address": "assigned@outlook.com",\n    "mailbox_provider": "outlook",\n    "requested_providers": ["outlook", "hotmail"],\n    "price": 0.60,\n    "status": "waiting_code",\n    "code": "",\n    "expires_at": "2026-08-18T12:30:00Z"\n  }\n}`;
  const serviceResponse = `{\n  "data": [{\n    "code": "openai",\n    "name": "OpenAI",\n    "description": "OpenAI 注册验证码",\n    "allowed_providers": ["outlook", "hotmail", "gmail", "icloud", "mailcom"],\n    "provider_prices": {\n      "outlook": 0.60,\n      "hotmail": 0.70,\n      "gmail": 0.90,\n      "icloud": 1.00,\n      "mailcom": 0.80\n    },\n    "ttl_seconds": 1800\n  }],\n  "pagination": { "page": 1, "page_size": 20, "total": 1, "total_pages": 1 }\n}`;
  publicContent.innerHTML = pageShell(
    "开发者文档",
    "用 API 管理注册收码任务",
    "所有接口均返回 JSON。API Key 只在用户门户创建弹窗中展示；请使用完整密钥，列表前缀不能调用接口。邮箱凭证、完整邮件和内部规则永不通过 API 返回。",
    `<div class="docs-layout">
      <nav class="docs-nav"><a href="#concept">接口约定</a><a href="#auth">鉴权</a><a href="#services">服务列表</a><a href="#create">创建订单</a><a href="#status-api">查询验证码</a><a href="#webhook">Webhook</a><a href="#errors">错误处理</a></nav>
      <div class="docs-content">
        <section id="concept"><h2>接口约定</h2><p>基础路径为 <code>/api/v1</code>。列表接口统一使用 <code>page</code> 和 <code>page_size</code> 服务端分页。创建订单即扣费并自动开始收码；30 分钟未收到验证码自动退款。</p></section>
        <section id="auth"><h2>鉴权</h2><p>在用户门户的“开发者”页面创建 API Key。每次请求都要携带 Bearer 令牌，也兼容登录后的会话令牌。</p><pre class="public-code">Authorization: Bearer hm_your_api_key</pre></section>
        <section id="services"><h2>服务列表</h2><p>获取当前可申请的目标平台。<code>provider_prices</code> 按邮箱类型给出价格；公开接口不返回库存、邮箱地址或内部匹配规则。</p><pre class="public-code">${escPublic(serviceExample)}</pre><p>响应示例：</p><pre class="public-code">${escPublic(serviceResponse)}</pre></section>
        <section id="create"><h2>创建订单</h2><p>提交目标平台代码，并通过 <code>mailbox_providers</code> 指定一个或多个可接受的邮箱类型。不能指定具体邮箱地址。余额需覆盖所选类型中的最高价格，系统分配后只按实际 <code>mailbox_provider</code> 的价格扣费。</p><pre class="public-code">${escPublic(createExample)}</pre><p>成功响应：</p><pre class="public-code">${escPublic(responseExample)}</pre><p><code>request_id</code> 用于幂等创建；同一用户重复提交相同值不会重复扣款。</p></section>
        <section id="status-api"><h2>查询订单与验证码</h2><pre class="public-code">GET /api/v1/orders/{id}\nAuthorization: Bearer hm_your_api_key</pre><p>订单创建后直接进入 <code>waiting_code</code>，后台自动匹配邮件；收到验证码后变为 <code>code_received</code>。用户端只查看进度和结果，超时退款、完成和取消均由后台处理。</p></section>
        <section id="webhook"><h2>Webhook</h2><p>在用户门户创建 Webhook 端点后，订单状态变化会通过 HMAC 签名推送。投递失败会指数退避，可在门户查看记录并手动重试。</p></section>
        <section id="errors"><h2>错误处理</h2><p>失败响应统一为 <code>{ "error": "错误码", "message": "可读说明" }</code>。常见错误包括 <code>401</code> 未授权、<code>402</code> 余额不足、<code>404</code> 订单或服务不存在、<code>409</code> 请求重复或订单状态冲突、<code>503</code> 暂无可分配邮箱。</p></section>
      </div>
    </div>`,
  );
}

function renderOpenSource() {
  publicContent.innerHTML = pageShell("页面已下线", "部署说明暂未公开", "当前公开页面提供产品介绍、价格和 API 文档。账户注册与任务申请请从工作台开始。", `<div class="docs-content"><section><h2>继续使用 HeroMail</h2><p>你可以查看公开价格、阅读 API 文档，或直接登录工作台管理注册收码任务。</p><div class="hero-actions"><a class="solid-action" href="/register">创建账户 <span aria-hidden="true">↗</span></a><a class="outline-action" href="/pricing">查看公开价格</a></div></section></div>`);
}

function renderAuth(register) {
  const title = register ? "创建 HeroMail 账户" : "登录 HeroMail";
  publicContent.innerHTML = `<section class="auth-page"><div class="auth-box"><a class="public-brand" href="/"><img src="/brand-mark.svg" alt=""><strong>HeroMail</strong></a><h1>${title}</h1><p>${register ? "注册后即可申请邮箱，并按需创建 API Key 与 Webhook。" : "登录后申请邮箱、查看收码任务并管理账户余额。"}</p><form id="public-auth-form" class="auth-form"><label>邮箱<input name="email" type="email" autocomplete="email" required></label>${register ? `<label>显示名称<input name="display_name" autocomplete="name"></label>` : ""}<label>密码<input name="password" type="password" minlength="10" autocomplete="${register ? "new-password" : "current-password"}" required></label><button type="submit">${register ? "注册并进入门户" : "登录"}</button></form><div class="auth-switch">${register ? `已有账户？<a href="/login">返回登录</a>` : `还没有账户？<a href="/register">创建账户</a>`}</div></div></section>`;
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
else if (path === "/open-source") renderOpenSource();
else if (path === "/login") renderAuth(false);
else if (path === "/register") renderAuth(true);
else if (path === "/") initHomeAnimations();
resolvePublicSession();
