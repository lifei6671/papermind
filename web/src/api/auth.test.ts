import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createAuthAPI } from "./auth";

describe("authApi", () => {
  test("平台管理员登录会映射服务端会话字段", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        username: "admin",
        password: "papermind123",
      });

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          access_token: "platform-access-token",
          refresh_token: "platform-refresh-token",
          user: {
            user_id: 1,
            display_name: "admin",
            role: "platform_admin",
          },
        },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    const session = await api.platformLogin({ username: "admin", password: "papermind123" });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/auth/platform/login",
      expect.objectContaining({ method: "POST" }),
    );
    expect(session).toEqual({
      accessToken: "platform-access-token",
      refreshToken: "platform-refresh-token",
      user: {
        userID: 1,
        displayName: "admin",
        role: "platform_admin",
      },
    });
  });

  test("租户码注册会提交租户码和用户基础信息", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_code: "PM-QT01",
        username: "student01",
        real_name: "张同学",
        password: "papermind123",
        phone: "13800002001",
        email: "student01@example.test",
      });

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 20,
          tenant_id: 10,
          username: "student01",
          real_name: "张同学",
          avatar_url: "",
          role: "student",
          status: "enabled",
        },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    const user = await api.tenantRegister({
      tenantCode: "PM-QT01",
      username: "student01",
      realName: "张同学",
      password: "papermind123",
      phone: "13800002001",
      email: "student01@example.test",
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/auth/tenant/register",
      expect.objectContaining({ method: "POST" }),
    );
    expect(user).toEqual({
      id: 20,
      tenantID: 10,
      username: "student01",
      realName: "张同学",
      avatarURL: "",
      role: "student",
      status: "enabled",
    });
  });
});
