import { describe, expect, test, vi } from "vitest";
import { ApiError, createApiClient, formatApiErrorMessage } from "./client";

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
      credentials: "include",
      headers: { Accept: "application/json" },
      method: "GET",
    });
  });

  test("请求默认携带 cookie credentials 且不注入 Bearer token", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "ok", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      }),
    );

    const client = createApiClient({
      baseUrl: "http://api.test",
      fetcher,
    });

    await client.post("/api/v1/platform/tenants", { name: "一中" });

    expect(fetcher).toHaveBeenCalledWith("http://api.test/api/v1/platform/tenants", {
      body: JSON.stringify({ name: "一中" }),
      credentials: "include",
      headers: {
        Accept: "application/json",
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

  test("非 2xx 响应保留后端业务错误码和统一错误提示文案", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 40001, message: "tenant_id 必须是正整数", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 400,
      }),
    );

    const client = createApiClient({ baseUrl: "", fetcher });

    await expect(client.get("/api/v1/exams?tenant_id=abc")).rejects.toMatchObject({
      code: 40001,
      message: "tenant_id 必须是正整数",
      status: 400,
    });
  });

  test("非 JSON 错误响应直接透出后端文本", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response("404 page not found", {
        headers: { "Content-Type": "text/plain" },
        status: 404,
      }),
    );

    const client = createApiClient({ baseUrl: "", fetcher });

    await expect(client.get("/api/v1/papers/101")).rejects.toMatchObject({
      code: 404,
      message: "404 page not found",
      status: 404,
      body: "404 page not found",
    });
  });

  test("统一错误提示在后端文案缺失时按错误码兜底", () => {
    const error = new ApiError("", 50000, 500, { code: 50000 });

    expect(formatApiErrorMessage(error, "默认失败")).toBe("服务暂时不可用，请稍后重试");
    expect(formatApiErrorMessage("unknown", "默认失败")).toBe("默认失败");
  });

  test("题库不足内部错误统一转换为中文提示", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 40001, message: "question pool insufficient", data: null }), {
        headers: { "Content-Type": "application/json" },
        status: 400,
      }),
    );

    const client = createApiClient({ baseUrl: "", fetcher });

    await expect(client.post("/api/v1/papers/100/rule-fixed/generate", { tenant_id: 10 })).rejects.toMatchObject({
      code: 40001,
      message: "题库题量不足，请调整题型题量、知识点范围或难度分布后重试",
      status: 400,
    });
  });

  test("完整 JSON 字符串错误先提取 message 再格式化", () => {
    const error = new Error(JSON.stringify({ code: 40001, message: "question pool insufficient", data: null }));

    expect(formatApiErrorMessage(error, "智能组卷生成失败")).toBe("题库题量不足，请调整题型题量、知识点范围或难度分布后重试");
  });
});
