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
