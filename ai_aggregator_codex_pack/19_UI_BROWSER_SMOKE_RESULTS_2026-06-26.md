# UI Browser Smoke Results - 2026-06-26

文档名称：`19_UI_BROWSER_SMOKE_RESULTS_2026-06-26.md`  
测试方式：Codex in-app Browser + rendered UI checks  
执行时间：2026-06-26 Asia/Dubai  
环境：`http://localhost:5175`, backend `http://localhost:8081`

---

## 1. 本次目标

补齐上一轮报告中记录的 UI 点击级验证缺口：

```text
app loads
→ login/register flow works
→ dashboard pages render
→ key controls respond
→ mobile viewport does not clip core dashboard pages
```

---

## 2. 覆盖范围

| 页面 / Flow | 验证内容 | 结果 |
|---|---|---|
| `/login` | 切换 Sign up，填写 username/email/password，Create Account | Pass |
| `/dashboard` | 注册后自动跳转 Dashboard，显示 metrics/balance/empty states | Pass |
| `/dashboard/request-logs` | 页面渲染，Success filter 点击后进入选中状态 | Pass |
| `/dashboard/files` | 页面渲染，搜索、purpose、upload controls 可见 | Pass |
| `/dashboard/workflows` | 页面渲染，点击 Create 后 Echo workflow 出现在列表 | Pass |
| `/playground` | 页面渲染，mode tabs、model selector、prompt、parameters 可见 | Pass |
| Dashboard identity / logout | Sidebar 显示真实 email/role，点击 Log out 后跳转 `/login` | Pass |
| `/admin` | 使用 admin 测试账号登录后加载 Admin Overview，无 403 | Pass |
| Mobile 390x844 `/dashboard/files` | Dashboard nav 不再固定挤压内容，Files 页面主内容占满视口宽度 | Pass after fix |

---

## 3. 发现的问题

### UI-001: Dashboard mobile layout clipped content

问题：

```text
390px mobile viewport 下，DashboardLayout 的 sidebar 仍以 fixed w-64 占据左侧，
Files 页面主内容被挤压到很窄区域，标题和卡片严重换行/裁切。
```

影响：

```text
Dashboard / Files / Workflows / Request Logs 等登录后页面在手机宽度不可用。
```

修复：

```text
frontend/src/components/layout/DashboardLayout.tsx
  desktop: keep fixed left sidebar
  mobile: use full-width top nav with horizontal scrolling
  main content: remove ml-64 on mobile, keep md:ml-64 on desktop
  user block hidden on mobile to preserve space
```

### UI-002: Dashboard used static user identity and had no logout action

问题：

```text
Dashboard sidebar 固定显示 user@example.com / Bronze，且 LogOut 图标不可点击。
这导致测试和实际使用时无法确认当前用户身份，也无法干净切换 admin 用户执行 Admin UI 验证。
```

修复：

```text
frontend/src/components/layout/DashboardLayout.tsx
  load /api/user/profile on mount
  show real email and role
  add accessible Log out button
  clear JWT/API key and navigate to /login
```

---

## 4. 修复后证据

Mobile viewport 390x844:

```text
aside: width=390, height=124, x=0
main: width=390, x=0, y=124
h1 Files visible
Upload File control visible
No fixed 256px left sidebar on mobile
```

Workflows interaction:

```text
Click Create
Workflow List contains Echo workflow
Selected workflow detail shows Run Workflow, Runs, Last Cost, Run History
```

Request Logs interaction:

```text
Click success filter
success button class changes to selected state: bg-brand-600 text-white
```

Admin E2E:

```text
Create admin test user via API
Promote role in local database
Log out from current dashboard session
Log in through UI with admin account
Open /admin
Admin Overview metrics render
No "Admin access required" message
```

---

## 5. Console / Runtime Notes

Relevant app runtime errors:

```text
None after page reload and rendered checks.
```

Known non-blocking warnings:

```text
React Router v7 future flag warnings from react-router-dom.
```

Temporary HMR errors appeared while copying updated frontend source into the running container:

```text
[vite] Failed to reload /src/pages/*.tsx
```

These were caused by live dev-server reload during container source sync. `npm run build` passed and fresh page loads rendered correctly after container restart.

---

## 6. Commands / Checks

```bash
cd frontend && npm run build
docker cp frontend/src/. aag-frontend:/app/src
docker restart aag-frontend
scripts/dev-check.sh
```

Build result:

```text
TypeScript + Vite build = Pass
Vite chunk size warning only
```

Service check:

```text
backend /health = HTTP 200
frontend = HTTP 200
MOCK_PROVIDER_MODE=false
```

---

## 7. Remaining UI Risk

| Risk | Status |
|---|---|
| Admin deep E2E | Completed for Admin Overview after adding logout/profile display |
| File upload binary chooser | Controls visible; actual browser file selection/upload not executed in this run |
| Playground real generation from UI | UI rendered; real API generation already covered by backend smoke, not re-run via browser UI |
| Full mobile coverage | Files page verified at 390x844; other dashboard pages should receive follow-up mobile checks |

---

## 8. 结论

UI browser smoke 已完成基础闭环，并修复了移动端 Dashboard 裁切和静态用户身份/logout 缺失两个实际可用性问题。当前登录、Dashboard、Request Logs、Files、Workflows、Playground、Admin Overview 的基础渲染和关键交互均已验证。
