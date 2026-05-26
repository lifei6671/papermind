import { describe, expect, test, vi } from "vitest";
import { ApiError, createApiClient } from "./client";

describe("api client", () => {
  test("成功响应只把 data 返回给业务页面", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "ok", data: { id: 7 } }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      }),
    );

    const client = createApiClient({ baseUrl: "http://api.test", fetcher });

    await expect(client.get<{ id: number }>("/api/v1/tenants/7")).resolves.toEqual({ id: 7 });
    expect(fetcher).toHaveBeenCalledWith("http://api.test/api/v1/tenants/7", {
      headers: { Accept: "application/json" },
      method: "GET",
    });
  });

  test("登录态存在时自动注入 Bearer token", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "ok", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      }),
    );

    const client = createApiClient({
      baseUrl: "http://api.test",
      fetcher,
      getAccessToken: () => "access-token",
    });

    await client.post("/api/v1/platform/tenants", { name: "一中" });

    expect(fetcher).toHaveBeenCalledWith("http://api.test/api/v1/platform/tenants", {
      body: JSON.stringify({ name: "一中" }),
      headers: {
        Accept: "application/json",
        Authorization: "Bearer access-token",
        "Content-Type": "application/json",
      },
      method: "POST",
    });
  });

  test("业务错误转成 ApiError 并保留错误码", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 1001, message: "租户不存在", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      }),
    );

    const client = createApiClient({ baseUrl: "", fetcher });

    await expect(client.get("/api/v1/tenants/404")).rejects.toMatchObject({
      code: 1001,
      message: "租户不存在",
      status: 200,
    });
  });

  test("HTTP 401 会触发未登录回调", async () => {
    const onUnauthorized = vi.fn();
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 401, message: "unauthorized", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 401,
      }),
    );

    const client = createApiClient({ baseUrl: "", fetcher, onUnauthorized });

    await expect(client.get("/api/v1/profile")).rejects.toBeInstanceOf(ApiError);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });
});
