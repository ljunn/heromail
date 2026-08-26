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

function renderLegal(kind) {
  const privacy = kind === "privacy";
  const title = privacy ? "隐私政策" : "服务条款";
  document.title = title + " - HeroMail";
  const kicker = privacy ? "数据说明 / 公开版" : "使用规则 / 公开版";
  const description = privacy
    ? "本政策说明 HeroMail 如何处理账户、邮箱连接和注册收码任务中的数据。它只覆盖 HeroMail 服务本身，不替代第三方平台的隐私政策。"
    : "使用 HeroMail 前请阅读这些规则。它们说明账户、订单、付款、邮箱资产和可接受的使用范围。";
  const sections = privacy ? [
    ["1. 我们处理哪些数据", "<p>创建账户时，我们处理邮箱地址、显示名称和经过加密保护的密码摘要。使用工作台时，我们还会记录订单、余额流水、支付状态、Webhook 投递记录和必要的审计信息。</p><p>当管理员接入邮箱时，系统可能保存邮箱地址、连接方式、授权凭证或应用专用密码。凭证使用服务端密钥加密保存，不会返回给用户，也不会写入日志。</p>"],
    ["2. 邮箱和邮件的使用边界", "<p>HeroMail 只为完成目标平台注册收码任务读取必要的邮件。用户端只能查看自己订单租约期间、命中该目标平台规则的相关邮件和验证码，不能查看其他平台邮件、完整收件箱、密码或 OAuth Token。</p><p>邮件匹配结果会用于推进订单、结算、超时退款和邮箱平台占用状态；不用于出售或构建个人画像。</p>"],
    ["3. OAuth 授权", "<p>使用 Google Gmail 或 Microsoft OAuth 时，登录和授权发生在对应平台的官方页面。HeroMail 不接收第三方账户密码或 2FA 地址，只保存完成收件所需的授权结果，并在连接失效时提示重新授权或切换支持的收件方式。</p>"],
    ["4. 数据保留与删除", "<p>订单、余额和审计记录会在满足结算、风控和合规需要的期限内保留。匹配邮件正文只在完成任务和处理争议所需的期限内保留，之后按系统保留策略清理。</p><p>你可以停止使用账户并向服务运营方申请删除仍可删除的数据。法律要求保留的交易或审计记录可能无法立即删除。</p>"],
    ["5. 安全措施", "<p>HeroMail 使用访问控制、加密存储、服务端分页和审计日志保护账户与邮箱资产。没有任何网络服务可以保证绝对安全；如果发现疑似未授权访问，请尽快通过服务运营方提供的支持渠道联系。</p>"],
    ["6. 第三方服务", "<p>Google、Microsoft、支付服务商和目标平台注册服务由各自的运营方提供。你使用这些服务时，还需要遵守它们各自的条款和隐私政策；HeroMail 不控制第三方的处理方式或可用性。</p>"],
    ["7. 政策更新", "<p>当数据处理方式或服务能力发生实质变化时，我们会在本页更新版本日期和说明。继续使用服务表示你已看到更新后的政策。</p>"]
  ] : [
    ["1. 服务范围", "<p>HeroMail 提供邮箱资源分配、目标平台注册收码、验证码提取、订单查询和相关 API。邮箱是平台资产，用户只能选择目标平台和可接受的邮箱类型，不能指定或长期占有某个具体邮箱。</p>"],
    ["2. 账户与安全", "<p>你应提供真实、可控制的账户邮箱，并妥善保管登录凭据和 API Key。API Key 只在创建时完整展示；如怀疑泄露，应立即吊销并重新创建。</p><p>一个账户的操作由账户持有人负责。不得共享账户、绕过权限或尝试访问其他用户的订单、邮件和余额。</p>"],
    ["3. 订单、计费与退款", "<p>创建订单会按最终分配的邮箱类型扣费并开始收码。收到验证码后订单进入后台结算流程，用户不能主动取消；超过订单有效期仍未收到匹配验证码时，系统按规则自动回收邮箱并退款。</p><p>公开价格、可选邮箱类型和订单有效期以创建订单时页面或 API 返回为准。支付渠道的到账时间可能受第三方影响。</p>"],
    ["4. 允许的使用方式", "<p>HeroMail 仅用于你有权进行的注册、集成和测试。你不得将服务用于撞库、冒用身份、绕过第三方风控、批量滥用、发送垃圾邮件、违法活动或侵犯他人权益的行为。</p><p>你还必须遵守目标平台的服务条款、开发者政策和适用法律。</p>"],
    ["5. 邮箱和邮件访问", "<p>分配邮箱仅限对应订单的注册收码任务使用。你不得尝试获取邮箱密码、OAuth Token、其他平台邮件或未授权数据，也不得通过 API、浏览器或其他方式绕过平台隔离。</p>"],
    ["6. 服务可用性与处置", "<p>邮箱连接失效、第三方接口故障、网络中断或维护可能导致任务延迟。我们会按产品规则处理超时、退款和失败状态，但不承诺第三方服务持续可用。</p><p>如果账户违反本条款、危及系统安全或产生异常风险，服务运营方可以限制功能、暂停订单或终止账户，并保留必要的审计记录。</p>"],
    ["7. 内容与知识产权", "<p>HeroMail 的软件、界面、文档和品牌由其权利人拥有。你提交的账户信息、订单参数和合法业务数据仍归你或相应权利人所有；你授予我们处理这些数据以提供服务的必要权限。</p>"],
    ["8. 条款更新与联系", "<p>我们会在本页发布条款更新。继续使用服务表示接受更新后的条款。若你对账户、订单或数据有疑问，请通过服务运营方提供的支持渠道联系。</p>"]
  ];
  const nav = sections.map(section => "<a href=\"#legal-" + section[0].split(".")[0] + "\">" + escPublic(section[0]) + "</a>").join("");
  const content = sections.map(section => "<section id=\"legal-" + section[0].split(".")[0] + "\"><h2>" + escPublic(section[0]) + "</h2>" + section[1] + "</section>").join("");
  const body = "<div class=\"legal-layout\"><aside class=\"legal-nav\"><span>本页内容</span>" + nav + "</aside><div class=\"docs-content legal-content\"><div class=\"legal-meta\"><span>生效日期</span><strong>2026 年 8 月 26 日</strong><span>适用于</span><strong>HeroMail 网页、API 和收码服务</strong></div>" + content + "</div></div>";
  publicContent.innerHTML = pageShell(kicker, title, description, body);
}

function renderAuth(register) {
  const title = register ? "创建 HeroMail 账户" : "登录 HeroMail";
  publicContent.innerHTML = `<section class="auth-page"><div class="auth-box"><a class="public-brand" href="/"><img src="/brand-mark.svg" alt=""><strong>HeroMail</strong></a><h1>${title}</h1><p>${register ? "注册后即可申请邮箱，并按需创建 API Key 与 Webhook。" : "登录后申请邮箱、查看收码任务并管理账户余额。"}</p><form id="public-auth-form" class="auth-form"><label>邮箱<input name="email" type="email" autocomplete="email" required></label>${register ? `<label>显示名称<input name="display_name" autocomplete="name"></label>` : ""}<label>密码<input name="password" type="password" minlength="10" autocomplete="${register ? "new-password" : "current-password"}" required></label><button type="submit">${register ? "注册并进入门户" : "登录"}</button></form><div class="auth-switch">${register ? `已有账户？<a href="/login">返回登录</a>` : `还没有账户？<a href="/register">创建账户</a>`}</div></div></section>`;
  document.querySelector("#public-auth-form").addEventListener("submit", event => submitAuth(event, register));
  if (register) document.querySelector(".auth-box").insertAdjacentHTML("beforeend", '<p class="auth-legal">注册前请阅读 <a href="/privacy">隐私政策</a> 和 <a href="/terms">服务条款</a>。</p>');
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
else if (path === "/privacy") renderLegal("privacy");
else if (path === "/terms") renderLegal("terms");
else if (path === "/login") renderAuth(false);
else if (path === "/register") renderAuth(true);
else if (path === "/") initHomeAnimations();
resolvePublicSession();
