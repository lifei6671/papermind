import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createExamAPI } from "./exams";

describe("examApi", () => {
  test("考试入口按邀请码解析考试信息", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        invite_code: "PM2026",
        user_id: 20,
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 1,
          tenant_id: 10,
          paper_id: 100,
          name: "高一语文期中考试",
          start_time: 1779792000000,
          end_time: 1779799200000,
          duration_minutes: 120,
          max_attempts: 1,
          result_strategy: "latest",
          publish_mode: "manual_publish",
          invite_code: "PM2026",
          status: "published",
        },
      }));
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const exam = await api.resolveInvite({ inviteCode: "PM2026", userID: 20 });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/exams/invite/resolve",
      expect.objectContaining({ method: "POST" }),
    );
    expect(exam).toEqual({
      id: 1,
      tenantID: 10,
      paperID: 100,
      name: "高一语文期中考试",
      paperName: "高一语文月考试卷",
      inviteCode: "PM2026",
      target: "未配置",
      status: "published",
      startAt: "2026-05-26 18:40",
      endAt: "2026-05-26 20:40",
      durationMinutes: 120,
    });
  });
});
