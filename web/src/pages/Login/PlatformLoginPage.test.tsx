import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { PlatformLoginPage } from "./PlatformLoginPage";
import type { AuthAPI } from "../../api/auth";

afterEach(() => {
  window.localStorage.clear();
});

test("平台管理员登录成功后保存登录态并进入概览", async () => {
  const user = userEvent.setup();
  const api: AuthAPI = {
    platformLogin: async (input) => {
      expect(input).toEqual({ username: "admin", password: "papermind123" });
      return {
        accessToken: "platform-access-token",
        refreshToken: "platform-refresh-token",
        user: {
          displayName: "admin",
          role: "platform_admin",
          userID: 1,
        },
      };
    },
    tenantRegister: vi.fn(),
  };

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route path="/login" element={<PlatformLoginPage api={api} />} />
          <Route path="/" element={<div>考试平台概览</div>} />
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.type(screen.getByLabelText("账号"), "admin");
  await user.type(screen.getByLabelText("密码"), "papermind123");
  await user.click(screen.getByRole("button", { name: "登录平台" }));

  expect(screen.getByText("考试平台概览")).toBeInTheDocument();
  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toContain("platform-access-token");
});

test("平台管理员登录失败时展示服务端错误且不保存登录态", async () => {
  const user = userEvent.setup();
  const api: AuthAPI = {
    platformLogin: async () => {
      throw new Error("账号或密码不正确");
    },
    tenantRegister: vi.fn(),
  };

  render(
    <SessionProvider>
      <MemoryRouter>
        <PlatformLoginPage api={api} />
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.type(screen.getByLabelText("账号"), "admin");
  await user.type(screen.getByLabelText("密码"), "wrong-password");
  await user.click(screen.getByRole("button", { name: "登录平台" }));

  expect(screen.getByRole("alert")).toHaveTextContent("账号或密码不正确");
  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toBeNull();
});

test("平台管理员登录页拒绝空账号", async () => {
  const user = userEvent.setup();

  render(
    <SessionProvider>
      <MemoryRouter>
        <PlatformLoginPage />
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.click(screen.getByRole("button", { name: "登录平台" }));

  expect(screen.getByRole("alert")).toHaveTextContent("请输入平台管理员账号");
});
