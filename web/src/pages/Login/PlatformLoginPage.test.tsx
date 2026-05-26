import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { PlatformLoginPage } from "./PlatformLoginPage";

afterEach(() => {
  window.localStorage.clear();
});

test("平台管理员登录成功后保存登录态并进入概览", async () => {
  const user = userEvent.setup();

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route path="/login" element={<PlatformLoginPage />} />
          <Route path="/" element={<div>考试平台概览</div>} />
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );

  await user.type(screen.getByLabelText("账号"), "admin");
  await user.type(screen.getByLabelText("密码"), "papermind123");
  await user.click(screen.getByRole("button", { name: "登录平台" }));

  expect(screen.getByText("考试平台概览")).toBeInTheDocument();
  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toContain("平台管理员");
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
