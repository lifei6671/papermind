import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { adminRoutes } from "../../app/routes";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { AdminShell } from "./AdminShell";

afterEach(() => {
  window.localStorage.clear();
});

test("退出登录时清理已保存登录态", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      accessToken: "access-token",
      refreshToken: "refresh-token",
      user: { displayName: "平台管理员", role: "platform_admin", userID: 1 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
            <Route path="/login" element={<div>登录页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "退出登录" }));

  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toBeNull();
});

test("租户业务导航保留当前目标租户 ID", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      accessToken: "access-token",
      refreshToken: "refresh-token",
      user: { displayName: "空间管理员", role: "space_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/spaces?tenant_id=10"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/spaces" element={<div>空间页</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByRole("link", { name: /用户管理/ })).toHaveAttribute("href", "/users?tenant_id=10");
});

test("平台管理员不展示租户空间菜单", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      accessToken: "access-token",
      refreshToken: "refresh-token",
      user: { displayName: "平台管理员", role: "platform_admin", userID: 1 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.queryByText("租户空间")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /空间管理/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /用户管理/ })).not.toBeInTheDocument();
});

test("空间管理员展示租户空间菜单", () => {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      accessToken: "access-token",
      refreshToken: "refresh-token",
      user: { displayName: "空间管理员", role: "space_admin", tenantID: 10, userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AdminShell routes={adminRoutes} />}>
            <Route path="/" element={<div>概览页面</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  expect(screen.getByText("租户空间")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /空间管理/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /用户管理/ })).toBeInTheDocument();
});
