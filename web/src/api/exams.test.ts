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

  test("答题 API 封装开始考试、保存答案和交卷请求", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/exams/1/attempts/start")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10, user_id: 20 });
        return jsonResponse({
          attempt: { id: 99, answer_deadline: 1779795600000 },
          exam_token: "exam-token",
          questions: [{
            id: 9001,
            sort_order: 1,
            section: { name: "一、单项选择题", instructions: "每题 2 分" },
            question: { title: "下列选项正确的是（ ）", type: "single" },
            options: [{ id: 101, key: "A", content: "正确选项" }],
            score: "2",
          }],
        });
      }
      if (url.endsWith("/api/v1/exam-attempts/99/answers/9001")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          exam_token: "exam-token",
          question_type: "single",
          option_ids: [101],
          text: "",
        });
        return jsonResponse({ saved: true, updated_at: 1779792000000 });
      }
      if (url.endsWith("/api/v1/exam-attempts/99/submit")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          exam_token: "exam-token",
          event_type: "submit",
        });
        return jsonResponse({ submitted: true });
      }
      throw new Error(`unexpected request: ${url}`);
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const started = await api.startAttempt({ tenantID: 10, examID: 1, userID: 20 });
    await api.saveAnswer({
      tenantID: 10,
      attemptID: started.attemptID,
      attemptQuestionID: started.questions[0].id,
      examToken: started.examToken,
      questionType: "single",
      optionIDs: [101],
    });
    await api.submitAttempt({ tenantID: 10, attemptID: started.attemptID, examToken: started.examToken });

    expect(started).toEqual({
      attemptID: 99,
      examToken: "exam-token",
      answerDeadline: 1779795600000,
      questions: [{
        id: 9001,
        number: 1,
        sectionTitle: "一、单项选择题",
        sectionSubtitle: "每题 2 分",
        type: "single",
        stem: "下列选项正确的是（ ）",
        score: 2,
        options: [{ id: 101, key: "A", content: "正确选项" }],
      }],
    });
  });
});

function jsonResponse(data: unknown) {
  return new Response(JSON.stringify({ code: 0, message: "ok", data }));
}
