"use client";

import {
  ArrowRight,
  BarChart3,
  Bot,
  CalendarDays,
  ClipboardList,
  ChevronDown,
  ChevronRight,
  CircleCheck,
  Database,
  FileSpreadsheet,
  Filter,
  Layers3,
  LineChart,
  LockKeyhole,
  Megaphone,
  MessageSquareText,
  PackageSearch,
  SearchCheck,
  ShieldCheck,
  RefreshCw,
  SlidersHorizontal,
  Sparkles,
  Target,
  Upload,
  Users,
  X,
} from "lucide-react";
import Image from "next/image";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { AivionLogo, AivionMark } from "@/components/brand/aivion-logo";
import { cn } from "@/components/ui/cn";

const FAQ_ITEMS = [
  {
    q: "Зачем Aivion, если уже есть Ozon Analytics, MPSTATS, Excel или BI?",
    a: "Эти инструменты помогают получать, рассчитывать и изучать данные. Aivion занимает следующий слой: анализирует конкретный кабинет, выделяет то, что требует внимания, превращает анализ в Действия и позволяет делегировать дополнительные исследования AI-чату.",
  },
  {
    q: "Нужно ли подключать свой кабинет, чтобы посмотреть Aivion?",
    a: "Нет. Demo проходит на демонстрационном Ozon-кабинете — подключать свой кабинет, готовить API-доступы или выгрузки заранее не нужно.",
  },
  {
    q: "Что нужно, чтобы подключить свой кабинет?",
    a: "Подключение проходит через Ozon: вы выбираете нужный кабинет и подтверждаете доступ Aivion. Логин и пароль передавать Aivion не нужно.",
  },
  {
    q: "Может ли Aivion сам менять цены, остатки или рекламные настройки?",
    a: "Нет. В текущей модели Aivion работает как аналитический ассистент: показывает ситуации и формирует Действия, но не изменяет цены, остатки или рекламные настройки самостоятельно.",
  },
  {
    q: "Какие данные анализирует Aivion?",
    a: "Aivion работает с доступными данными Ozon-кабинета и объединяет их в согласованный аналитический контекст. Конкретный набор областей зависит от доступности данных и возможностей кабинета.",
  },
  {
    q: "Что будет, если часть данных Ozon недоступна?",
    a: "Недоступность одной области не должна блокировать остальные. Aivion показывает, какие данные доступны, и продолжает работать с теми областями, для которых достаточно данных.",
  },
  {
    q: "Что можно поручить AI-чату?",
    a: "Можно разбирать найденные ситуации, поручать отдельный анализ, обсуждать Действия и добавлять свои файлы. Результатом может быть ответ, таблица или подготовленный рабочий файл.",
  },
  {
    q: "Что происходит после Demo?",
    a: "Если Aivion подходит вашей команде, следующим шагом можно перейти к 14-дневному пилоту на собственном Ozon-кабинете и проверить продукт уже на данных бизнеса.",
  },
] as const;

const CHAT_CASES = [
  {
    title: "Разобраться в ситуации",
    subtitle: "Понять, почему изменился показатель",
    prompt: "Какие области сильнее всего повлияли на изменение заказов?",
    response: "Aivion использует контекст кабинета и сопоставляет связанные области, чтобы выделить факторы, которые требуют проверки.",
    icon: SearchCheck,
  },
  {
    title: "Провести анализ",
    subtitle: "Сравнить товары, категории или кампании",
    prompt: "Сделай анализ за период и выдели объекты, которые требуют внимания.",
    response: "Результат можно структурировать в сравнительную таблицу и продолжить исследование прямо в диалоге.",
    icon: BarChart3,
  },
  {
    title: "Обсудить Действие",
    subtitle: "Продолжить анализ с уже переданным контекстом",
    prompt: "Почему Aivion предлагает это Действие?",
    response: "Контекст Действия уже находится в чате: не нужно заново объяснять, какие объекты и данные обсуждаются.",
    icon: MessageSquareText,
  },
  {
    title: "Добавить свои данные",
    subtitle: "Совместить файл команды с данными Ozon",
    prompt: "Сравни план из файла с фактическими данными и подготовь результат.",
    response: "Aivion объединяет пользовательский файл с доступным контекстом кабинета и может вернуть готовую таблицу или XLSX.",
    icon: Upload,
  },
] as const;

const ACTIONS = [
  ["Пополнить запас по ключевым товарам", "Остатки", PackageSearch],
  ["Проверить рекламные кампании", "Реклама", Megaphone],
  ["Разобрать изменение заказов", "Продажи", LineChart],
  ["Проверить ценовые ситуации", "Цены", Target],
] as const;

const DASHBOARD_TABS = ["Обзор", "Воронка", "Остатки", "Реклама"] as const;

const DEMO_KPIS = [
  { label: "Заказано на сумму", value: "3 782 981 ₽", delta: "+12,4%", tone: "positive" },
  { label: "Заказано товаров", value: "12 842", delta: "+8,7%", tone: "positive" },
  { label: "Доля рекламы", value: "18,6%", delta: "−2,1 п.п.", tone: "neutral" },
] as const;

function LandingBrandLogo({ compact = false, className = "" }: { compact?: boolean; className?: string }) {
  return (
    <Image
      src="/landing/aivion-logo-lockup.png"
      alt="Aivion"
      width={1195}
      height={301}
      priority={!compact}
      className={cn("landing-brand-logo", compact && "landing-brand-logo-compact", className)}
    />
  );
}

type DemoModalProps = { open: boolean; onClose: () => void };

function DemoModal({ open, onClose }: DemoModalProps) {
  const [sent, setSent] = useState(false);

  useEffect(() => {
    const previousBodyOverflow = document.body.style.overflow;
    const previousHtmlOverflow = document.documentElement.style.overflow;
    const previousBodyOverscroll = document.body.style.overscrollBehavior;

    if (open) {
      document.body.style.overflow = "hidden";
      document.documentElement.style.overflow = "hidden";
      document.body.style.overscrollBehavior = "none";
    }

    return () => {
      document.body.style.overflow = previousBodyOverflow;
      document.documentElement.style.overflow = previousHtmlOverflow;
      document.body.style.overscrollBehavior = previousBodyOverscroll;
    };
  }, [open]);

  if (!open) return null;

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSent(true);
  }

  return (
    <div className="landing-modal-backdrop" role="presentation" onMouseDown={onClose}>
      <div
        className="landing-modal-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="demo-modal-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="landing-modal-geometry" aria-hidden="true" />
        <button className="landing-modal-close" onClick={onClose} aria-label="Закрыть">
          <X size={23} strokeWidth={1.8} />
        </button>
        {sent ? (
          <div className="landing-success-state">
            <LandingBrandLogo className="landing-modal-logo" />
            <span className="landing-success-icon"><CircleCheck size={34} /></span>
            <h2 id="demo-modal-title">Заявка отправлена</h2>
            <p>Свяжемся с вами и подберём удобное время для Демо.</p>
            <p className="landing-muted">Ничего подключать или готовить заранее не нужно.</p>
            <button className="landing-primary-button" onClick={onClose}>Закрыть</button>
          </div>
        ) : (
          <>
            <div className="landing-modal-brand"><LandingBrandLogo className="landing-modal-logo" /></div>
            <h2 id="demo-modal-title">Посмотреть Aivion в работе</h2>
            <p className="landing-modal-lead">Оставьте контакты — свяжемся с вами и подберём удобное время для 30-минутного Демо.</p>
            <div className="landing-modal-reassurance"><CircleCheck size={20} /><span>Ничего подключать или готовить заранее не нужно</span></div>
            <form onSubmit={submit} className="landing-demo-form">
              <label>
                <span className="landing-field-label">Имя*</span>
                <input name="name" required placeholder="Как к вам обращаться" />
              </label>
              <label>
                <span className="landing-field-label">Телефон*</span>
                <input name="phone" required type="tel" placeholder="+7 999 000-00-00" />
              </label>
              <label>
                <span className="landing-field-row"><span className="landing-field-label">Компания / бренд</span><span className="landing-field-optional">необязательно</span></span>
                <input name="company" placeholder="Название компании" />
              </label>
              <button className="landing-primary-button landing-modal-submit" type="submit">Записаться на Демо <ArrowRight size={19} /></button>
              <p className="landing-form-note">Нажимая кнопку, вы соглашаетесь с <a href="/privacy">политикой обработки персональных данных</a>.</p>
            </form>
          </>
        )}
      </div>
    </div>
  );
}

function ProductSkeleton({ mode = "Обзор", compact = false }: { mode?: string; compact?: boolean }) {
  return (
    <div className={cn("landing-product-ui", compact && "landing-product-ui-compact")}>
      <aside className="landing-product-sidebar">
        <LandingBrandLogo compact />
        <div className="landing-product-nav">
          {["Обзор", "Воронка", "Остатки", "Реклама"].map((item, index) => (
            <span key={item} className={index === 0 ? "is-active" : ""}>{item}</span>
          ))}
        </div>
        <div className="landing-product-nav-footer"><span>Действия</span><span>AI-чат</span></div>
      </aside>
      <div className="landing-product-main">
        <div className="landing-product-topline">
          <strong>{mode}</strong><span>Демо-кабинет Ozon</span>
        </div>
        <div className="landing-kpi-grid">
          {DEMO_KPIS.map((item) => (
            <div className="landing-kpi-card" key={item.label}>
              <span>{item.label}</span><strong>{item.value}</strong><small className={`is-${item.tone}`}>{item.delta} <em>к пред. периоду</em></small><i />
            </div>
          ))}
        </div>
        <div className="landing-overview-grid">
          <div className="landing-ai-summary-card">
            <b><Sparkles size={15} /> AI-сводка</b>
            <div className="landing-ai-summary-copy">
              <p>Заказы растут относительно предыдущего периода, но часть роста сдерживается остатками ключевых SKU.</p>
              <p>Доля рекламы снизилась, однако несколько кампаний требуют проверки.</p>
              <p>В фокусе — товары с низким покрытием и изменения эффективности рекламы.</p>
            </div>
            <span className="landing-summary-link">Смотреть детали в AI-чате <ArrowRight size={13}/></span>
          </div>
          <div className="landing-focus-card">
            <div className="landing-card-title"><b>Фокус на сегодня</b><span>3</span></div>
            {ACTIONS.slice(0, 3).map(([title, label, Icon]) => (
              <div className="landing-focus-row" key={title}>
                <span><Icon size={15}/></span><div><b>{title}</b><small>{label}</small></div><ChevronRight size={15}/>
              </div>
            ))}
          </div>
        </div>
        {!compact && (
          <div className="landing-product-lower-grid">
            <div><span>Динамика и изменения</span><div className="landing-line-chart"><i/><i/><i/><i/><i/></div></div>
            <div><span>Области для проверки</span><div className="landing-mini-bars"><i/><i/><i/><i/></div></div>
          </div>
        )}
      </div>
    </div>
  );
}

function ProductFlowSection() {
  const sectionRef = useRef<HTMLElement | null>(null);
  const trackRef = useRef<HTMLDivElement | null>(null);
  const [step, setStep] = useState(0);

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    let raf = 0;
    const update = () => {
      raf = 0;
      const rect = track.getBoundingClientRect();
      const stickyTop = 78;
      const stickyHeight = Math.max(1, window.innerHeight - stickyTop);
      const travel = Math.max(1, track.offsetHeight - stickyHeight);
      const progress = Math.min(1, Math.max(0, (stickyTop - rect.top) / travel));
      const next = progress < .18 ? 0 : progress < .38 ? 1 : progress < .62 ? 2 : progress < .82 ? 3 : 4;
      setStep((prev) => prev === next ? prev : next);
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  const steps = [
    ["Обзор", "Общая картина бизнеса и ключевые метрики"],
    ["Находит ситуацию", "AI анализирует данные и выявляет отклонения"],
    ["Формирует Действие", "Предлагает конкретные шаги для решения"],
    ["Спросить Aivion", "Можно уточнить детали в AI-чате"],
    ["Результат", "Готовый рабочий материал для команды"],
  ] as const;

  return (
    <section ref={sectionRef} id="product-flow" className="landing-product-flow">
      <div className="landing-container landing-product-flow-header">
        <span className="landing-eyebrow">КАК ЭТО РАБОТАЕТ</span>
        <h2>Aivion сам находит, что требует внимания — и формирует конкретные Действия</h2>
        <p>От общей картины бизнеса к найденной ситуации, конкретному Действию, дополнительному анализу в AI-чате и готовому результату.</p>
      </div>
      <div ref={trackRef} className="landing-product-flow-track">
        <div className="landing-product-flow-sticky">
          <div className="landing-container landing-flow-layout">
            <div className="landing-flow-steps" aria-label="Этапы сценария">
              {steps.map(([label, description], index) => (
                <div key={label} className={cn("landing-flow-step", step === index && "is-active", step > index && "is-done")}>
                  <span>{String(index + 1).padStart(2, "0")}</span>
                  <div><b>{label}</b><small>{description}</small></div>
                </div>
              ))}
            </div>
            <div className="landing-flow-stage-card">
              {step <= 1 && (
                <div className={cn("landing-flow-screen", step === 1 && "landing-flow-screen-focused")}>
                  <ProductSkeleton compact />
                  {step === 1 && <div className="landing-floating-insight"><span><PackageSearch size={18}/></span><div><b>Найдена ситуация, требующая внимания</b><small>Здесь позже будет реальная ситуация из Canonical Demo Account</small></div></div>}
                </div>
              )}
              {step === 2 && <ActionDetailMock />}
              {step === 3 && <ActionChatMock />}
              {step === 4 && <ResultMock />}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function ActionDetailMock() {
  return (
    <div className="landing-action-scene">
      <div className="landing-context-rail"><span><PackageSearch size={18}/></span><b>Найденная ситуация</b><p>Контекст сохраняется и превращается в конкретное Действие.</p></div>
      <div className="landing-action-detail-card">
        <span className="landing-demo-pill">ДЕЙСТВИЕ</span>
        <h3>Конкретный следующий шаг для команды</h3>
        <div className="landing-action-detail-grid"><div><span>Что сделать</span><p>Здесь будет формулировка Действия из Canonical Demo Account.</p></div><div><span>Почему</span><p>Короткое объяснение на фактах, которые стоят за рекомендацией.</p></div></div>
        <div className="landing-objects-strip"><span>Объекты</span><i/><i/><i/></div>
        <button className="landing-primary-button"><Bot size={16}/> Спросить Aivion</button>
      </div>
    </div>
  );
}

function ActionChatMock() {
  return (
    <div className="landing-action-chat-scene">
      <div className="landing-chat-context"><span className="landing-demo-pill">КОНТЕКСТ ДЕЙСТВИЯ</span><b>Не нужно объяснять ситуацию заново</b><p>Объекты и данные уже переданы в AI-чат.</p></div>
      <div className="landing-chat-window">
        <div className="landing-chat-window-head"><AivionMark size="small"/><b>AI-чат</b><span>Контекст кабинета</span></div>
        <div className="landing-user-bubble">Почему Aivion предлагает это Действие?</div>
        <div className="landing-ai-bubble"><Sparkles size={16}/><p>Разбираю ситуацию по связанным данным и выделяю факторы, которые стоит проверить.</p></div>
        <div className="landing-chat-input">Задайте вопрос или поручите анализ… <ArrowRight size={15}/></div>
      </div>
    </div>
  );
}

function ResultMock() {
  return (
    <div className="landing-result-scene">
      <div className="landing-result-copy"><span className="landing-eyebrow">РАБОЧИЙ РЕЗУЛЬТАТ</span><h3>Анализ не заканчивается на тексте</h3><p>Результат можно продолжить в диалоге или получить в структурированном виде.</p></div>
      <div className="landing-result-card"><FileSpreadsheet size={34}/><div><b>Аналитический результат.xlsx</b><span>Таблица · будет сформирована на demo-данных</span></div><button>Скачать</button></div>
    </div>
  );
}

function ChatSection() {
  const sectionRef = useRef<HTMLElement | null>(null);
  const [active, setActive] = useState(0);

  useEffect(() => {
    const section = sectionRef.current;
    if (!section) return;
    let raf = 0;
    const update = () => {
      raf = 0;
      const rect = section.getBoundingClientRect();
      const travel = Math.max(1, section.offsetHeight - window.innerHeight);
      const progress = Math.min(1, Math.max(0, -rect.top / travel));
      const next = Math.min(3, Math.floor(progress * 4));
      setActive((prev) => prev === next ? prev : next);
    };
    const onScroll = () => { if (!raf) raf = requestAnimationFrame(update); };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  const item = CHAT_CASES[active];
  const ActiveIcon = item.icon;

  return (
    <section ref={sectionRef} id="ai-chat" className="landing-chat-section landing-scroll-stage landing-chat-scroll-stage">
      <div className="landing-sticky-stage landing-chat-sticky">
        <div className="landing-container landing-chat-layout">
          <div className="landing-chat-copy">
            <span className="landing-eyebrow">AI-ЧАТ</span>
            <h2>Поручайте анализ AI, который уже знает ваш Ozon-кабинет</h2>
            <p>Один workspace для вопросов, полноценного анализа, обсуждения Действий и работы с собственными файлами.</p>
            <div className="landing-chat-cases">
              {CHAT_CASES.map((chatCase, index) => {
                const Icon = chatCase.icon;
                return <button key={chatCase.title} className={cn("landing-chat-case", active === index && "is-active")} onClick={() => setActive(index)}><span>{String(index + 1).padStart(2, "0")}</span><Icon size={18}/><div><b>{chatCase.title}</b><small>{chatCase.subtitle}</small></div></button>;
              })}
            </div>
          </div>
          <div className="landing-chat-workspace">
            <div className="landing-chat-window-head"><AivionMark size="small"/><b>AI-чат</b><span>Контекст кабинета</span></div>
            <div className="landing-chat-workspace-body" key={active}>
              <div className="landing-user-bubble"><ActiveIcon size={15}/>{item.prompt}</div>
              <div className="landing-ai-bubble landing-ai-bubble-large"><Sparkles size={17}/><div><b>Aivion</b><p>{item.response}</p>{active === 3 ? <div className="landing-attachment-row"><FileSpreadsheet size={18}/><div><b>План_продаж.xlsx</b><span>Файл пользователя + данные Ozon</span></div></div> : <div className="landing-analysis-placeholder"><i/><i/><i/><i/></div>}</div></div>
            </div>
            <div className="landing-chat-input">Задайте вопрос или опишите задачу… <ArrowRight size={15}/></div>
          </div>
        </div>
      </div>
    </section>
  );
}

function DashboardSection() {
  const [tab, setTab] = useState<(typeof DASHBOARD_TABS)[number]>("Обзор");
  const visual = useMemo(() => {
    if (tab === "Обзор") return <DashboardOverviewMock />;
    if (tab === "Воронка") return <FunnelMock />;
    if (tab === "Остатки") return <StocksMock />;
    return <AdvertisingMock />;
  }, [tab]);

  const tabIcon = {
    "Обзор": BarChart3,
    "Воронка": Filter,
    "Остатки": PackageSearch,
    "Реклама": Megaphone,
  } as const;

  return (
    <section id="dashboard" className="landing-section landing-dashboard-section">
      <div className="landing-container">
        <div className="landing-section-intro landing-section-intro-centered"><span className="landing-eyebrow">ДАННЫЕ ПОД КОНТРОЛЕМ</span><h2>Открыли Aivion — сразу видите, что изменилось и что важно</h2><p>Сначала — ключевая картина. Детали — в отдельных областях, когда они действительно нужны.</p></div>
        <div className="landing-dashboard-tabs" role="tablist">{DASHBOARD_TABS.map((item) => { const Icon = tabIcon[item]; return <button key={item} role="tab" aria-selected={tab === item} onClick={() => setTab(item)} className={tab === item ? "is-active" : ""}><Icon size={18}/><span>{item}</span></button>; })}</div>
        <div className="landing-dashboard-stage" key={tab}>{visual}</div>
      </div>
    </section>
  );
}

function DashboardOverviewMock() {
  return (
    <div className="landing-dashboard-overview">
      <aside className="landing-dashboard-overview-sidebar">
        <LandingBrandLogo compact />
        <nav>
          <span className="is-active"><BarChart3 size={17}/>Обзор</span>
          <span><Filter size={17}/>Воронка</span>
          <span><PackageSearch size={17}/>Остатки</span>
          <span><Megaphone size={17}/>Реклама</span>
        </nav>
        <div className="landing-dashboard-sidebar-bottom">
          <span><MessageSquareText size={17}/>AI-чат</span>
          <div className="landing-dashboard-account"><i>AK</i><div><b>Аккаунт</b><small>Демо · Ozon</small></div><ChevronRight size={15}/></div>
        </div>
      </aside>
      <div className="landing-dashboard-overview-main">
        <div className="landing-dashboard-overview-head">
          <div><h3>Обзор</h3><p>Ключевые показатели вашего Ozon-бизнеса</p></div>
          <div className="landing-dashboard-overview-controls">
            <button><CalendarDays size={15}/><span>1 — 31 авг. 2026</span><ChevronDown size={14}/></button>
            <button><i className="landing-ozon-dot">OZ</i><span>Ozon</span><ChevronDown size={14}/></button>
          </div>
        </div>

        <div className="landing-dashboard-kpis">
          {DEMO_KPIS.map((item, index) => (
            <div className="landing-dashboard-kpi" key={item.label}>
              <div className="landing-dashboard-kpi-title"><span>{item.label}</span><i><BarChart3 size={16}/></i></div>
              <strong>{item.value}</strong>
              <small className={index === 2 ? "is-negative" : "is-positive"}>{item.delta} <em>к пред. периоду</em></small>
              <div className={`landing-dashboard-spark landing-dashboard-spark-${index + 1}`}><i/><i/><i/><i/><i/><i/></div>
            </div>
          ))}
        </div>

        <div className="landing-dashboard-insight-grid">
          <div className="landing-dashboard-ai-card">
            <div className="landing-dashboard-ai-card-head"><b><Sparkles size={20}/>AI-сводка</b><span><i/>Обновлено 2 часа назад</span></div>
            <p>Продажи выросли на 12,4% по сравнению с предыдущим периодом.<br/>Основной рост обеспечили категории «Дом и интерьер» и «Красота».<br/>Обратите внимание на снижение доли рекламы и рост остатков в категории «Электроника» — это может влиять на оборачиваемость.</p>
            <button>Смотреть детали в AI-чате <ArrowRight size={14}/></button>
          </div>
          <div className="landing-dashboard-focus-card">
            <div className="landing-card-title"><b>Фокус на сегодня</b><span>3</span></div>
            <div className="landing-dashboard-focus-row"><span className="is-green"><ArrowRight size={16}/></span><div><b>Пополнить запас по ключевым товарам</b><small>12 товаров с низким остатком</small></div><ChevronRight size={15}/></div>
            <div className="landing-dashboard-focus-row"><span><Megaphone size={16}/></span><div><b>Проверить рекламные кампании</b><small>3 кампании с ростом CPA</small></div><ChevronRight size={15}/></div>
            <div className="landing-dashboard-focus-row"><span><PackageSearch size={16}/></span><div><b>Проанализировать остатки</b><small>В 5 категориях растут остатки</small></div><ChevronRight size={15}/></div>
          </div>
        </div>

        <div className="landing-dashboard-lower-grid">
          <div className="landing-dashboard-chart-card"><div className="landing-dashboard-card-head"><b>Динамика заказов</b><button>По дням <ChevronDown size={13}/></button></div><div className="landing-dashboard-chart"><i/><i/><i/><i/><i/><i/><i/><i/></div></div>
          <div className="landing-dashboard-category-card"><div className="landing-dashboard-card-head"><b>Топ категории по заказам</b><button>Все категории</button></div><div className="landing-dashboard-category-row"><span><PackageSearch size={15}/></span><b>Дом и интерьер</b><strong>28,4%</strong><i><em/></i></div></div>
        </div>
      </div>
    </div>
  );
}

function FunnelMock() {
  return <div className="landing-domain-mock"><div className="landing-domain-sidebar"><AivionLogo size="small"/><b>Воронка</b><p>Большая классическая воронка строится только из совместимых backend-confirmed этапов.</p></div><div className="landing-funnel-visual">{[100,82,60,43,29].map((width, index) => <div key={width} style={{width: `${width}%`}}><span>Этап {index + 1}</span></div>)}</div><div className="landing-domain-side-card"><b>Что изменилось</b><div className="landing-placeholder-lines"><i/><i/><i/><i/></div></div></div>;
}
function StocksMock() {
  return <div className="landing-domain-mock"><div className="landing-domain-sidebar"><AivionLogo size="small"/><b>Остатки</b><p>В фокусе — приоритетные SKU и покрытие, а не бесконечная таблица ассортимента.</p></div><div className="landing-stock-list">{[1,2,3,4].map((item)=><div key={item}><span className="landing-stock-icon"><PackageSearch size={18}/></span><div><b>Приоритетный SKU</b><small>Покрытие и динамика</small></div><span className="landing-stock-bar"><i style={{width: `${30 + item*12}%`}}/></span></div>)}</div><div className="landing-domain-side-card"><b>Требуют внимания</b><div className="landing-placeholder-lines"><i/><i/><i/></div></div></div>;
}
function AdvertisingMock() {
  return <div className="landing-domain-mock"><div className="landing-domain-sidebar"><AivionLogo size="small"/><b>Реклама</b><p>Ключевые показатели, изменения и кампании, которые стоит проверить.</p></div><div className="landing-ad-chart"><div className="landing-line-chart landing-line-chart-large"><i/><i/><i/><i/><i/><i/></div><div className="landing-ad-campaigns">{[1,2,3].map(i=><div key={i}><Megaphone size={17}/><span>Кампания {String(i).padStart(2,"0")}</span><b>Проверить</b></div>)}</div></div><div className="landing-domain-side-card"><b>Связанные Действия</b><div className="landing-placeholder-lines"><i/><i/><i/></div></div></div>;
}

export default function LandingShell() {
  const [demoOpen, setDemoOpen] = useState(false);
  const [faqOpen, setFaqOpen] = useState<number | null>(0);
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 48);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  const openDemo = () => setDemoOpen(true);

  return (
    <main className="landing-root">
      {demoOpen && <DemoModal open onClose={() => setDemoOpen(false)} />}
      <header className={cn("landing-header", scrolled && "is-scrolled")}>
        <div className="landing-container landing-header-inner">
          <a href="#top" aria-label="Aivion — на главную"><LandingBrandLogo /></a>
          <nav className="landing-nav" aria-label="Навигация по лендингу">
            <a href="#product-flow">Продукт</a><a href="#for-whom">Для кого</a><a href="#connection">Подключение</a><a href="#faq">FAQ</a>
          </nav>
          <button className="landing-header-cta" onClick={openDemo}>Посмотреть Aivion в работе</button>
        </div>
      </header>

      <section id="top" className="landing-hero">
        <div className="landing-hero-glow landing-hero-glow-one"/><div className="landing-hero-glow landing-hero-glow-two"/>
        <div className="landing-hero-geometry landing-hero-geometry-left" aria-hidden="true"><i/><i/><i/></div>
        <div className="landing-hero-geometry landing-hero-geometry-right" aria-hidden="true"><i/><i/><i/></div>
        <div className="landing-container landing-hero-inner">
          <div className="landing-hero-copy">
            <h1>AI-ассистент для<br/><span>Ozon-бизнеса</span></h1>
            <p>Aivion анализирует данные Ozon-кабинета, собирает их в целостную картину бизнеса и показывает, что требует внимания. На основе анализа сервис формирует конкретные Действия для команды, а AI-чат помогает продолжать исследование данных, выполнять аналитические задачи и глубже разбирать найденные ситуации.</p>
            <button className="landing-primary-button landing-hero-cta" onClick={openDemo}>Посмотреть Aivion в работе <ArrowRight size={17}/></button>
            <div className="landing-hero-microcopy" aria-label="Условия демонстрации"><span>30 минут</span><i/> <span>бесплатно</span><i/> <span>демонстрационный Ozon-кабинет</span><i/> <span>без подключения вашего кабинета</span></div>
          </div>
          <div className="landing-hero-product-wrap"><div className="landing-hero-product-aura"/><ProductSkeleton /></div>
        </div>
      </section>

      <ProductFlowSection />

      <section className="landing-section landing-positioning-section">
        <div className="landing-positioning-atmosphere landing-positioning-atmosphere-blue" aria-hidden="true" />
        <div className="landing-positioning-atmosphere landing-positioning-atmosphere-cyan" aria-hidden="true" />
        <div className="landing-positioning-corner-mark" aria-hidden="true" />
        <div className="landing-container landing-positioning-container">
          <div className="landing-section-intro landing-section-intro-centered landing-positioning-intro">
            <span className="landing-eyebrow">ПОЧЕМУ AIVION</span>
            <h2>Данные уже есть. Aivion берёт на себя аналитическую работу.</h2>
            <p>Ozon Analytics, MPSTATS, Excel и BI дают данные.<br/>Aivion помогает понять, что делать дальше.</p>
          </div>

          <div className="landing-positioning-story">
            <div className="landing-positioning-panel landing-positioning-sources">
              <h3>У команды уже есть<br/>сервисы для сбора данных</h3>
              <div className="landing-positioning-service-list">
                <div><span className="landing-service-icon landing-service-ozon">OZON</span><b>Ozon Analytics</b></div>
                <div><span className="landing-service-icon landing-service-mpstats"><BarChart3 size={22}/></span><b>MPSTATS</b></div>
                <div><span className="landing-service-icon landing-service-excel"><FileSpreadsheet size={22}/></span><b>Excel</b></div>
                <div><span className="landing-service-icon landing-service-bi"><BarChart3 size={22}/></span><b>BI / другие сервисы</b></div>
              </div>
            </div>

            <div className="landing-positioning-problem">
              <div className="landing-positioning-bridge" aria-hidden="true"><i/><span><SlidersHorizontal size={24}/></span><i/></div>
              <h3>Проблема<br/>не в доступе<br/>к данным —<br/><em>а в работе<br/>с ними.</em></h3>
              <span className="landing-positioning-problem-rule" aria-hidden="true" />
            </div>

            <div className="landing-positioning-panel landing-positioning-next-step">
              <h3><span>Aivion выполняет следующий шаг.</span><br/>Помогает понять, что делать дальше</h3>
              <div className="landing-positioning-outcome-list">
                <div><span><SearchCheck size={25}/></span><b>Показывает, что изменилось</b></div>
                <div><span><BarChart3 size={25}/></span><b>Выделяет, что важно</b></div>
                <div><span><Layers3 size={25}/></span><b>Формирует Действия</b></div>
                <div><span><Sparkles size={25}/></span><b>Встроенный AI для любых задач</b></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <ChatSection />

      <section className="landing-section landing-actions-section">
        <div className="landing-container">
          <div className="landing-actions-intro">
            <div className="landing-actions-intro-copy"><span className="landing-eyebrow">ДЕЙСТВИЯ</span><h2>Aivion превращает анализ<br/>в конкретные Действия</h2><p>Aivion формирует конкретные шаги по товарам, категориям и рекламным кампаниям, объединяет связанные задачи и показывает, с чего начать.</p></div>
            <div className="landing-actions-benefits">
              <div><span><Database size={22}/></span><div><b>Основано<br/>на ваших данных</b><small>Продажи, остатки, реклама<br/>и динамика спроса</small></div></div>
              <div><span><Target size={22}/></span><div><b>Выделяет<br/>главное</b><small>Показывает, что влияет<br/>на результат сильнее всего</small></div></div>
              <div><span><CircleCheck size={22}/></span><div><b>Помогает<br/>действовать</b><small>Готовые шаги с пояснениями<br/>и контекстом</small></div></div>
            </div>
          </div>
          <div className="landing-actions-stage">
            <div className="landing-actions-list">
              <div className="landing-actions-list-head"><b>Действия</b><span>4</span></div>
              <button className="is-active"><span><PackageSearch size={18}/></span><div><b>Пополнить запас по ключевым товарам</b><small>Остатки · 12 товаров</small></div><ChevronRight size={17}/></button>
              <button><span><Megaphone size={18}/></span><div><b>Проверить рекламные кампании</b><small>Реклама · 3 кампании</small></div><ChevronRight size={17}/></button>
              <button><span><LineChart size={18}/></span><div><b>Разобрать изменение заказов</b><small>Продажи · 7 товаров</small></div><ChevronRight size={17}/></button>
              <button><span><Target size={18}/></span><div><b>Проверить ценовые ситуации</b><small>Цены · 5 товаров</small></div><ChevronRight size={17}/></button>
              <div className="landing-actions-personal"><span><Sparkles size={20}/></span><div><b>Это ваши персональные Действия</b><p>Они формируются на основе данных вашего кабинета и обновляются ежедневно.</p></div></div>
            </div>
            <div className="landing-actions-detail">
              <div className="landing-actions-detail-head"><span className="landing-demo-pill">ОТКРЫТО</span><small><i/>Обновлено 2 часа назад</small></div>
              <h3>Пополнить запас по ключевым товарам</h3>
              <p className="landing-actions-detail-lead">12 товаров с низким остатком. Есть риск упущенных продаж в ближайшие недели.</p>
              <div className="landing-actions-detail-grid-v2">
                <div className="landing-actions-info-card"><div className="landing-actions-info-title"><span><Layers3 size={16}/></span><b>ЧТО СДЕЛАТЬ</b></div><ol><li>Пополнить остатки по 12 товарам</li><li>Учесть текущую динамику продаж</li><li>Проверить сроки поставки и планировать с учётом акций</li></ol></div>
                <div className="landing-actions-info-card"><div className="landing-actions-info-title"><span><BarChart3 size={16}/></span><b>ПОЧЕМУ ЭТО ВАЖНО</b></div><ul><li>Текущих остатков хватит в среднем на 6 дней</li><li>Ожидается рост спроса в ближайшие 2 недели</li><li>Несколько ключевых SKU могут закончиться раньше срока</li><li>Рекомендуем пополнить запас до 15 сентября</li></ul></div>
              </div>
              <div className="landing-actions-products-head"><b>Товары в фокусе</b><button>Смотреть все 12 товаров <ArrowRight size={14}/></button></div>
              <div className="landing-actions-products">
                <div className="landing-actions-product-card"><span className="landing-product-thumb is-jar"><i/></span><div><b>Крем для лица<br/>увлажняющий</b><small>Артикул: 12345678</small><em>Осталось 8 шт.</em></div></div>
                <div className="landing-actions-product-card"><span className="landing-product-thumb is-bottle"><i/></span><div><b>Шампунь<br/>восстанавливающий</b><small>Артикул: 87654321</small><em>Осталось 12 шт.</em></div></div>
                <div className="landing-actions-product-card"><span className="landing-product-thumb is-diffuser"><i/></span><div><b>Аромадиффузор<br/>для дома</b><small>Артикул: 11223344</small><em>Осталось 15 шт.</em></div></div>
              </div>
              <div className="landing-actions-detail-footer"><div><span><Database size={18}/></span><div><b>Основано на ваших данных</b><small>Остатки · Продажи · Динамика спроса · Сезонность · Рекламные кампании</small></div></div><button className="landing-primary-button"><Bot size={16}/> Спросить Aivion <ArrowRight size={15}/></button></div>
            </div>
          </div>
        </div>
      </section>

      <DashboardSection />

      <section id="for-whom" className="landing-section landing-audience-section">
        <div className="landing-container">
          <div className="landing-section-intro landing-audience-intro"><span className="landing-eyebrow">ДЛЯ КОГО</span><h2>Aivion особенно полезен, когда Ozon — уже значимый канал продаж</h2><p>Когда в кабинете много товаров, работает реклама, идут регулярные поставки и Ozon занимается команда, главная проблема уже не в том, где посмотреть цифры.</p></div>
          <div className="landing-audience-grid landing-audience-grid-v2">
            <article className="landing-audience-card landing-audience-card-v2 landing-audience-card-01">
              <span className="landing-audience-number">01</span>
              <div className="landing-audience-card-copy"><h3>Ассортимент уже невозможно<br/>контролировать вручную</h3><p>Сотни SKU и постоянные изменения<br/>делают ручной контроль ненадёжным.</p><div className="landing-audience-tags"><span>SKU</span><span>Цены</span><span>Остатки</span></div></div>
              <div className="landing-audience-sku-list" aria-hidden="true">
                {[['SKU 102938','is-green'],['SKU 104572','is-amber'],['SKU 110093','is-red']].map(([sku,tone])=><div key={sku}><span><PackageSearch size={14}/></span><b>{sku}</b><i className={tone}/></div>)}
              </div>
            </article>

            <article className="landing-audience-card landing-audience-card-v2 landing-audience-card-02">
              <span className="landing-audience-number">02</span>
              <div className="landing-audience-card-copy"><h3>Реклама и остатки требуют<br/>постоянного контроля</h3><p>Команде приходится регулярно сверять<br/>несколько областей бизнеса.</p></div>
              <div className="landing-audience-branch" aria-hidden="true"><i/><span/><div><span><BarChart3 size={18}/></span><b>Реклама</b><em className="is-green"/></div><div><span><PackageSearch size={18}/></span><b>Остатки</b><em className="is-amber"/></div></div>
            </article>

            <article className="landing-audience-card landing-audience-card-v2 landing-audience-card-03 is-accent">
              <span className="landing-audience-number">03</span>
              <div className="landing-audience-card-copy"><h3>Аналитика уже есть — но выводы<br/>всё равно приходится делать вручную</h3><p>Данные доступны, но поиск причин и приоритетов<br/>остаётся на менеджере.</p></div>
              <div className="landing-audience-flow" aria-label="Данные — выводы — решения"><span><BarChart3 size={17}/><b>Данные</b></span><ArrowRight size={17}/><span><Sparkles size={17}/><b>Выводы</b></span><ArrowRight size={17}/><span><Target size={17}/><b>Решения</b></span></div>
            </article>

            <article className="landing-audience-card landing-audience-card-v2 landing-audience-card-04">
              <span className="landing-audience-number">04</span>
              <div className="landing-audience-card-copy"><h3>Кабинетом управляет команда,<br/>и нужен общий фокус</h3><p>Aivion помогает начинать день с одной картины<br/>и общей рабочей повестки.</p></div>
              <div className="landing-audience-focus-flow" aria-hidden="true"><div><span><BarChart3 size={16}/></span><b>Продажи</b></div><div><span><Megaphone size={16}/></span><b>Реклама</b></div><div><span><PackageSearch size={16}/></span><b>Поставки</b></div><i/><strong><Users size={16}/>Общий фокус</strong></div>
            </article>
          </div>
        </div>
      </section>

      <section id="connection" className="landing-section landing-connection-section landing-connection-section-v2">
        <div className="landing-container landing-connection-layout landing-connection-layout-v2">
          <div className="landing-connection-copy"><span className="landing-eyebrow">ПОДКЛЮЧЕНИЕ</span><h2>Подключите Ozon один раз — дальше Aivion работает с данными сам</h2><p>Подключение проходит через Ozon. После подтверждения доступа Aivion регулярно синхронизирует доступные данные и обновляет аналитику.</p><div className="landing-trust-list landing-trust-list-v2"><div><ShieldCheck size={22}/><span><b>Авторизация через Ozon</b><small>Логин и пароль передавать не нужно</small></span></div><div><LockKeyhole size={22}/><span><b>Только аналитический доступ</b><small>Aivion не меняет цены, остатки или рекламные настройки</small></span></div></div></div>
          <div className="landing-connection-card landing-connection-card-v2">
            <div className="landing-connection-brandflow"><div className="landing-connection-brand-tile landing-connection-ozon-tile">OZON</div><ArrowRight size={30}/><div className="landing-connection-brand-tile landing-connection-aivion-tile"><LandingBrandLogo compact/></div></div>
            <div className="landing-connection-steps-v2">
              <div className="landing-connection-step-v2"><span>1</span><b>Выберите кабинет<br/>в Ozon</b></div>
              <i/>
              <div className="landing-connection-step-v2"><span>2</span><b>Подтвердите доступ<br/>к аналитическим данным</b></div>
              <i/>
              <div className="landing-connection-step-v2 is-done"><span>3</span><b>Готово<br/>Данные обновляются автоматически</b></div>
            </div>
            <div className="landing-connection-sync"><span><RefreshCw size={24}/></span><b>Данные синхронизируются автоматически<br/><em>Ozon → Aivion · Безопасно и надежно</em></b></div>
          </div>
        </div>
      </section>

      <section className="landing-section landing-demo-section landing-demo-section-v2">
        <div className="landing-container">
          <div className="landing-demo-card landing-demo-card-v2">
            <div className="landing-demo-copy landing-demo-copy-v2">
              <span className="landing-eyebrow">ПОСМОТРИТЕ В РАБОТЕ</span>
              <h2>За 30 минут поймите, какую часть аналитической работы можно передать Aivion</h2>
              <p>Это не презентация функций, а рабочий сценарий на демонстрационном Ozon-кабинете: понять ситуацию, открыть Действие, продолжить анализ в AI-чате и получить результат.</p>
              <button className="landing-primary-button" onClick={openDemo}>Посмотреть Aivion в работе <ArrowRight size={18}/></button>
              <div className="landing-demo-micro landing-demo-micro-v2">
                <MessageSquareText size={28} strokeWidth={1.8}/>
                <div><b>Есть типовая задача вашей команды?</b><span>Возьмите её на Демо — разберём похожий сценарий.</span></div>
              </div>
            </div>
            <div className="landing-demo-timeline landing-demo-timeline-v2">
              {[
                { n: "01", t: "Обзор", d: "Покажем общую картину бизнеса", Icon: BarChart3 },
                { n: "02", t: "Действие", d: "Разберём конкретную ситуацию", Icon: ClipboardList },
                { n: "03", t: "AI-чат", d: "Продолжим дополнительный анализ", Icon: MessageSquareText },
                { n: "04", t: "Результат", d: "Получим готовый рабочий материал", Icon: LineChart },
              ].map(({n,t,d,Icon}, index)=>(
                <div key={n} className={cn("landing-demo-timeline-step", index===3 && "is-result")}>
                  <span>{n}</span><Icon size={27} strokeWidth={1.8}/><div><b>{t}</b><small>{d}</small></div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section id="faq" className="landing-section landing-faq-section landing-faq-section-v2">
        <div className="landing-container landing-faq-layout landing-faq-layout-v2">
          <div className="landing-faq-side landing-faq-side-v2">
            <span className="landing-eyebrow">FAQ</span>
            <h2>Ответы на вопросы перед Демо</h2>
            <p>Только то, что действительно важно перед первым знакомством с Aivion.</p>
          </div>
          <div className="landing-faq-list landing-faq-list-v2">
            {FAQ_ITEMS.map((item,index)=><div className={cn("landing-faq-item landing-faq-item-v2", faqOpen===index && "is-open")} key={item.q}><button onClick={()=>setFaqOpen(faqOpen===index?null:index)} aria-expanded={faqOpen===index}><span>{item.q}</span><ChevronDown size={18}/></button><div className="landing-faq-answer"><p>{item.a}</p></div></div>)}
          </div>
        </div>
      </section>

      <section className="landing-final-cta"><div className="landing-final-glow"/><div className="landing-container landing-final-inner"><AivionMark size="large"/><h2>Посмотрите Aivion в работе и решите, подходит ли он вашей команде</h2><p>За 30 минут покажем, как Aivion помогает понять состояние бизнеса, превращает анализ в Действия и выполняет аналитические задачи в AI-чате.</p><button className="landing-primary-button" onClick={openDemo}>Посмотреть Aivion в работе <ArrowRight size={16}/></button><div className="landing-final-micro">30 минут · бесплатно · демонстрационный Ozon-кабинет · без подключения вашего кабинета</div></div></section>

      <footer className="landing-footer"><div className="landing-container landing-footer-grid"><div><AivionLogo size="large"/><p>AI-ассистент для Ozon-бизнеса</p></div><div><b>Навигация</b><a href="#product-flow">Продукт</a><a href="#for-whom">Для кого</a><a href="#connection">Подключение</a><a href="#faq">FAQ</a></div><div><b>Документы</b><span>Политика конфиденциальности</span><span>Обработка персональных данных</span><span>Финальные ссылки — перед запуском</span></div><div><b>Demo</b><button onClick={openDemo}>Посмотреть Aivion в работе</button><span>30 минут · бесплатно</span></div></div><div className="landing-container landing-footer-bottom"><span>© 2026 Aivion</span><span>Desktop prototype · Landing Skeleton v1</span></div></footer>
    </main>
  );
}
