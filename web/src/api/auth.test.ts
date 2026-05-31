import { afterEach, describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { authApi, createAuthAPI } from "./auth";
import { SESSION_STORAGE_KEY } from "../auth/session-context";

afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
});

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

  test("租户用户登录会映射租户身份字段", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        username: "student20",
        password: "papermind123",
      });

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          user: {
            user_id: 20,
            display_name: "目标考生",
            role: "tenant_user",
          },
        },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    const session = await api.tenantLogin({ username: "student20", password: "papermind123" });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/auth/tenant/login",
      expect.objectContaining({ method: "POST" }),
    );
    expect(session).toEqual({
      user: {
        userID: 20,
        displayName: "目标考生",
        role: "tenant_user",
      },
    });
  });

  test("选择租户空间会提交空间上下文并映射绑定后的会话", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        space_id: 100,
      });

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          user: {
            user_id: 20,
            display_name: "目标考生",
            role: "student",
            tenant_id: 10,
          },
        },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    const session = await api.selectTenantSpace({ tenantID: 10, spaceID: 100 });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/auth/tenant/select-space",
      expect.objectContaining({ method: "POST" }),
    );
    expect(session.user).toEqual({
      userID: 20,
      displayName: "目标考生",
      role: "student",
      tenantID: 10,
    });
  });

  test("当前用户空间授权列表会映射空间成员关系字段", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("GET");

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 1,
            tenant_id: 10,
            tenant_name: "青藤一中",
            space_id: 100,
            space_name: "高一 1 班",
            role: "space_admin",
            status: "enabled",
          }],
        },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.listProfileSpaces();

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/profile/spaces",
      expect.objectContaining({ method: "GET" }),
    );
    expect(result.items).toEqual([{
      id: 1,
      tenantID: 10,
      tenantName: "青藤一中",
      spaceID: 100,
      spaceName: "高一 1 班",
      role: "space_admin",
      status: "enabled",
    }]);
  });

  test("退出登录会请求服务端清除 cookie 会话", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { logged_out: true },
      }));
    });
    const api = createAuthAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.logout();

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/auth/logout",
      expect.objectContaining({ method: "POST" }),
    );
  });

  test("默认 authApi 通过 cookie 发送受保护接口请求", async () => {
    window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({
      user: { userID: 20, displayName: "目标考生", role: "tenant_user" },
    }));
    const fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      expect(init?.credentials).toBe("include");
      expect(init?.headers).not.toHaveProperty("Authorization");
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: [] },
      }));
    });

    await authApi.listProfileSpaces();

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/profile/spaces",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
