import { afterEach, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createProfileAPI, profileApi } from "./profile";
import { SESSION_STORAGE_KEY } from "../auth/session-context";

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
});

test("profile API 读取并更新当前登录用户资料", async () => {
  const fetcher = vi.fn(async (input, init) => {
    if (input === "/api/v1/profile" && init?.method === "GET") {
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          user_id: 1,
          tenant_id: undefined,
          display_name: "admin",
          avatar_url: "",
          phone: "admin-phone",
          email: "admin@example.test",
          role: "platform_admin",
          subject_type: "platform_user",
        },
      }));
    }
    if (input === "/api/v1/profile" && init?.method === "POST") {
      expect(init.body).toBe(JSON.stringify({
        display_name: "平台负责人",
        avatar_url: "/uploads/avatars/admin.png",
        phone: "13800000001",
        email: "owner@example.test",
      }));
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          user_id: 1,
          display_name: "平台负责人",
          avatar_url: "/uploads/avatars/admin.png",
          phone: "13800000001",
          email: "owner@example.test",
          role: "platform_admin",
          subject_type: "platform_user",
        },
      }));
    }
    return new Response(JSON.stringify({ code: 50000, message: "unexpected request", data: null }), { status: 500 });
  });

  const api = createProfileAPI(createApiClient({ baseUrl: "", fetcher }));

  await expect(api.getProfile()).resolves.toMatchObject({
    displayName: "admin",
    role: "platform_admin",
  });
  await expect(api.updateProfile({
    displayName: "平台负责人",
    avatarURL: "/uploads/avatars/admin.png",
    phone: "13800000001",
    email: "owner@example.test",
  })).resolves.toMatchObject({
    displayName: "平台负责人",
    avatarURL: "/uploads/avatars/admin.png",
  });
});

test("默认 profileApi 会为受保护接口带上本地会话 token", async () => {
  window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({
    accessToken: "stored-access-token",
    refreshToken: "stored-refresh-token",
    user: { userID: 1, displayName: "admin", role: "platform_admin" },
  }));
  const fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
    expect(init?.headers).toMatchObject({
      Authorization: "Bearer stored-access-token",
    });
    return new Response(JSON.stringify({
      code: 0,
      message: "ok",
      data: {
        user_id: 1,
        display_name: "admin",
        avatar_url: "",
        phone: "",
        email: "",
        role: "platform_admin",
        subject_type: "platform_user",
      },
    }));
  });

  await profileApi.getProfile();

  expect(fetcher).toHaveBeenCalledWith(
    "/api/v1/profile",
    expect.objectContaining({ method: "GET" }),
  );
});
