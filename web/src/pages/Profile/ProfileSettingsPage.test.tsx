import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { SessionProvider } from "../../auth/session";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import type { ProfileAPI } from "../../api/profile";
import { ProfileSettingsPage } from "./ProfileSettingsPage";

afterEach(() => {
  window.localStorage.clear();
});

test("租户用户个人设置读取资料并保存后同步本地登录态名称", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({
    accessToken: "access-token",
    refreshToken: "refresh-token",
    user: { userID: 20, tenantID: 10, displayName: "张三", role: "teacher" },
  }));
  const api: ProfileAPI = {
    getProfile: vi.fn().mockResolvedValue({
      userID: 20,
      tenantID: 10,
      displayName: "张三",
      avatarURL: "",
      phone: "tenant-phone",
      email: "teacher@example.test",
      role: "teacher",
      subjectType: "tenant_user",
    }),
    updateProfile: vi.fn().mockResolvedValue({
      userID: 20,
      tenantID: 10,
      displayName: "李老师",
      avatarURL: "/uploads/avatars/teacher.png",
      phone: "13800000002",
      email: "teacher-new@example.test",
      role: "teacher",
      subjectType: "tenant_user",
    }),
  };

  render(
    <SessionProvider>
      <ProfileSettingsPage api={api} />
    </SessionProvider>,
  );

  expect(await screen.findByDisplayValue("张三")).toBeInTheDocument();
  await user.clear(screen.getByLabelText("显示名称"));
  await user.type(screen.getByLabelText("显示名称"), "李老师");
  await user.clear(screen.getByLabelText("头像地址"));
  await user.type(screen.getByLabelText("头像地址"), "/uploads/avatars/teacher.png");
  await user.clear(screen.getByLabelText("手机号"));
  await user.type(screen.getByLabelText("手机号"), "13800000002");
  await user.clear(screen.getByLabelText("邮箱"));
  await user.type(screen.getByLabelText("邮箱"), "teacher-new@example.test");
  await user.click(screen.getByRole("button", { name: "保存资料" }));

  await waitFor(() => expect(api.updateProfile).toHaveBeenCalledWith({
    displayName: "李老师",
    avatarURL: "/uploads/avatars/teacher.png",
    phone: "13800000002",
    email: "teacher-new@example.test",
  }));
  expect(screen.getByRole("status", { name: "profile-save-result" })).toHaveTextContent("已保存");
  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toContain("\"displayName\":\"李老师\"");
});

test("平台管理员个人设置保持登录账号只读", async () => {
  const user = userEvent.setup();
  window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({
    accessToken: "access-token",
    refreshToken: "refresh-token",
    user: { userID: 1, displayName: "admin", role: "platform_admin" },
  }));
  const api: ProfileAPI = {
    getProfile: vi.fn().mockResolvedValue({
      userID: 1,
      displayName: "admin",
      avatarURL: "",
      phone: "admin-phone",
      email: "admin@example.test",
      role: "platform_admin",
      subjectType: "platform_user",
    }),
    updateProfile: vi.fn().mockResolvedValue({
      userID: 1,
      displayName: "admin",
      avatarURL: "/uploads/avatars/admin.png",
      phone: "13800000001",
      email: "owner@example.test",
      role: "platform_admin",
      subjectType: "platform_user",
    }),
  };

  render(
    <SessionProvider>
      <ProfileSettingsPage api={api} />
    </SessionProvider>,
  );

  expect(await screen.findByLabelText("登录账号")).toBeDisabled();
  await user.clear(screen.getByLabelText("头像地址"));
  await user.type(screen.getByLabelText("头像地址"), "/uploads/avatars/admin.png");
  await user.clear(screen.getByLabelText("手机号"));
  await user.type(screen.getByLabelText("手机号"), "13800000001");
  await user.clear(screen.getByLabelText("邮箱"));
  await user.type(screen.getByLabelText("邮箱"), "owner@example.test");
  await user.click(screen.getByRole("button", { name: "保存资料" }));

  await waitFor(() => expect(api.updateProfile).toHaveBeenCalledWith({
    displayName: "admin",
    avatarURL: "/uploads/avatars/admin.png",
    phone: "13800000001",
    email: "owner@example.test",
  }));
  expect(window.localStorage.getItem(SESSION_STORAGE_KEY)).toContain("\"displayName\":\"admin\"");
});
