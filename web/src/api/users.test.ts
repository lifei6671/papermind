import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createUserAPI } from "./users";

describe("user api", () => {
  test("用户列表会映射租户用户字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 20,
            tenant_id: 10,
            username: "teacher01",
            real_name: "李老师",
            avatar_url: "",
            role: "teacher",
            status: "enabled",
            force_password_change: true,
            phone: "13800000020",
            email: "li@example.test",
            last_login_ip: "203.0.113.20",
            last_login_at: 1717297200000,
            created_at: 1717290000000,
            updated_at: 1717293600000,
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listUsers({
      tenantID: 10,
      page: 2,
      pageSize: 50,
      search: " 李老师 ",
      filters: { role: "student", status: "enabled", excludeSpaceID: 301 },
    })).resolves.toEqual({
      items: [{
        id: 20,
        tenantID: 10,
        name: "李老师",
        username: "teacher01",
        role: "teacher",
        avatarFileName: "未上传",
        status: "enabled",
        forcePasswordChange: true,
        phone: "13800000020",
        email: "li@example.test",
        lastLoginIP: "203.0.113.20",
        lastLoginAt: 1717297200000,
        createdAt: 1717290000000,
        updatedAt: 1717293600000,
      }],
      page: 1,
      pageSize: 20,
      total: 1,
    });
    expect(fetcher).toHaveBeenCalledWith("/api/v1/tenant/users?tenant_id=10&page=2&page_size=50&search=%E6%9D%8E%E8%80%81%E5%B8%88&role=student&status=enabled&exclude_space_id=301", expect.objectContaining({ method: "GET" }));
  });

  test("创建用户会提交租户和角色字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "enabled",
          force_password_change: true,
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createUser({
      tenantID: 10,
      name: "张三",
      username: "student01",
      password: "student-secure-123",
      role: "student",
      avatarFileName: "avatar.png",
      forcePasswordChange: true,
    })).resolves.toMatchObject({
      id: 21,
      tenantID: 10,
      name: "张三",
      role: "student",
      forcePasswordChange: true,
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          password: "student-secure-123",
          role: "student",
          force_password_change: true,
        }),
      }),
    );
  });

  test("编辑用户资料会提交姓名、手机号和邮箱", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 20,
          tenant_id: 10,
          username: "teacher01",
          real_name: "李老师新",
          avatar_url: "avatar.png",
          role: "teacher",
          status: "enabled",
          phone: "13800000021",
          email: "li-new@example.test",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.updateUser({
      tenantID: 10,
      userID: 20,
      name: "李老师新",
      phone: "13800000021",
      email: "li-new@example.test",
    })).resolves.toMatchObject({
      id: 20,
      name: "李老师新",
      phone: "13800000021",
      email: "li-new@example.test",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users/20/profile",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          tenant_id: 10,
          real_name: "李老师新",
          phone: "13800000021",
          email: "li-new@example.test",
        }),
      }),
    );
  });


  test("启用用户会携带操作者 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "enabled",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.enableUser({ tenantID: 10, actorID: 99, userID: 21 })).resolves.toMatchObject({
      id: 21,
      status: "enabled",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users/21/enable",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          actor_id: 99,
        }),
      }),
    );
  });

  test("禁用用户会携带操作者 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "disabled",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.disableUser({ tenantID: 10, actorID: 99, userID: 21 })).resolves.toMatchObject({
      id: 21,
      status: "disabled",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users/21/disable",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          actor_id: 99,
        }),
      }),
    );
  });
});
