# Aivion — Frontend Brand & UI Refresh

Документ описывает бренд-систему и UI-обновление frontend-проекта.

## Название продукта

**Aivion** — AI-сервис для продавцов Ozon.

User-facing название заменено с «Marketplace AI / MVP» на **Aivion**. Внутренние идентификаторы (package names, API, migrations) не менялись.

## Логотип

Компоненты расположены в:

- `frontend/components/brand/aivion-logo.tsx`

| Компонент | Описание |
|-----------|----------|
| `AivionLogo` | Горизонтальный lockup: знак + wordmark «Aivion» |
| `AivionMark` | Только геометрический знак (буква A из 3 элементов) |

Размеры: `small` / `default` / `large`. Prop `markOnly` для logo без текста.

### Favicon

- `frontend/app/icon.svg` — SVG-иконка на основе `AivionMark`
- Подключена через `metadata.icons` в `frontend/app/layout.tsx`

## Цветовая палитра

Централизована в `frontend/app/globals.css` через CSS-переменные и Tailwind v4 `@theme inline`.

| Токен | Значение | Назначение |
|-------|----------|------------|
| `--aivion-navy` | `#061B45` | Текст, структура, wordmark |
| `--aivion-blue` | `#0873F6` | Primary actions |
| `--aivion-turquoise` | `#00D1D8` | AI / accent |
| `--aivion-bg` | `#F7FAFC` | Фон приложения |
| `--aivion-surface` | `#FFFFFF` | Карточки |
| `--aivion-border` | `#E5EAF2` | Границы |
| `--aivion-text` | `#07152F` | Основной текст |
| `--aivion-muted` | `#667085` | Вторичный текст |
| `--aivion-success` | `#12B76A` | Успех |
| `--aivion-warning` | `#F79009` | Предупреждение |
| `--aivion-danger` | `#F04438` | Ошибка |

Использование в Tailwind: `bg-aivion-blue`, `text-aivion-muted`, `border-aivion-border` и т.д.

## Типографика

- Шрифт: **Manrope** (Google Fonts через `next/font/google`)
- Подключение: `frontend/app/layout.tsx`, CSS variable `--font-manrope`
- Система размеров: page title 28px, section 18px, body 14px, helper 12–14px muted

## UI-принципы

- Спокойный, минималистичный SaaS-стиль
- Карточки: white surface + thin border, radius 20–24px
- Мягкие тени, больше воздуха
- Единые loading / empty / error states
- Без MVP/debug/raw текстов в user-facing UI
- Focus-visible states на интерактивных элементах

## UI-kit (`frontend/components/ui/`)

| Компонент | Файл | Статус |
|-----------|------|--------|
| Button | `button.tsx` | Обновлён (primary = aivion-blue) |
| Card | `card.tsx` | Обновлён |
| Badge / StatusPill | `badge.tsx` | Обновлён |
| StatusBadge | `status-badge.tsx` | Обновлён |
| EmptyState | `empty-state.tsx` | Обновлён |
| LoadingState / Skeleton | `loading-state.tsx` | Обновлён |
| PageHeader / SectionHeader | `page-header.tsx` | Обновлён |
| MetricCard / ActionCard | `metric-card.tsx` | Обновлён |
| ErrorState / AlertBanner | `error-state.tsx` | Обновлён |
| Input / Textarea / FormField | `input.tsx` | Создан |
| Tabs | `tabs.tsx` | Создан |
| Logo | `../brand/aivion-logo.tsx` | Создан |

Дополнительно: `frontend/components/auth-layout.tsx` — брендовый layout для auth.

## Обновлённые экраны

| Экран | Компонент | Изменения |
|-------|-----------|-----------|
| App shell | `app-shell.tsx` | AivionLogo, nav icons, brand colors, reorder nav |
| Auth | `login/page.tsx`, `register/page.tsx` | AuthLayout, FormField, Input |
| Dashboard | `dashboard-screen.tsx` + `dashboard/*` | AlertBanner, brand tokens |
| Ozon connection | `ozon-onboarding.tsx` | PageHeader, AlertBanner, brand forms |
| Products | `products-screen.tsx`, `product-image.tsx` | Product cards; demo image fallback via `/demo/products/` |
| AI Chat | `chat-screen.tsx` | PageHeader, bubbles, empty states, hide trace |
| Alerts | `alerts-screen.tsx` | Brand tokens |
| Recommendations | `recommendations-screen.tsx` | Brand tokens |
| Advertising | `advertising-screen.tsx` | Brand tokens |
| Critical SKUs | `critical-skus-screen.tsx` | Brand tokens |
| Stocks | `stocks-replenishment-screen.tsx` | Brand tokens |

## App shell / Navigation

- Sidebar: `AivionLogo` (desktop), `AivionMark` (mobile header)
- Active nav: blue highlight `#eff8ff`
- Порядок: Дашборд → Товары → Реклама → Критические SKU → Остатки → Алерты → Рекомендации → AI-чат → Подключение Ozon
- Admin/support — только для admin-пользователей

## Что не менялось

- Backend API и бизнес-логика
- Маршруты и user flows
- Admin/support screens (только brand tokens)
- Dev/system pages (`/dev/*`)
- Package/repo names
- `MVP_RECOMMENDATION_TYPES` и другие internal code identifiers

## Проверки

```bash
cd frontend
npm run lint
npm run build
```
