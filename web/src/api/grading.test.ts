import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createGradingAPI } from "./grading";

describe("grading api", () => {
  test("读取待阅卷列表并映射后端字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            attempt_id: 900,
            attempt_question_id: 901,
            student_name: "张三",
            space_name: "高一 1 班",
            exam_name: "高一语文期中考试",
            question_title: "岳阳楼记思想内涵",
            answer_content: "先忧后乐体现了责任意识。",
            submitted_at: 1779792000000,
            max_score: "10",
            answer_version: 7,
            pending_short_text_count: 1,
            status: "pending",
          }],
        },
      })),
    );

    const api = createGradingAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listPendingAttempts({
      tenantID: 10,
      examID: 1,
      actorID: 501,
      actorRole: "teacher",
      spaceID: 301,
    })).resolves.toEqual({
      items: [{
        attemptID: 900,
        attemptQuestionID: 901,
        studentName: "张三",
        spaceName: "高一 1 班",
        examName: "高一语文期中考试",
        questionTitle: "岳阳楼记思想内涵",
        answerContent: "先忧后乐体现了责任意识。",
        submittedAt: "2026-05-26 18:40",
        maxScore: "10",
        answerVersion: 7,
        pendingShortTextCount: 1,
        status: "pending",
      }],
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/grading/pending?tenant_id=10&exam_id=1&actor_id=501&actor_role=teacher&space_id=301",
      expect.objectContaining({ method: "GET" }),
    );
  });

  test("保存简答题阅卷时携带 version", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ code: 0, message: "ok", data: { graded: true } })),
    );
    const api = createGradingAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.gradeShortText({
      tenantID: 10,
      examID: 1,
      attemptID: 900,
      attemptQuestionID: 901,
      actorID: 501,
      actorRole: "teacher",
      spaceID: 301,
      answerVersion: 7,
      score: "4.5",
      comment: "要点完整",
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/exam-attempts/900/questions/901/grade",
      expect.objectContaining({
        body: JSON.stringify({
          tenant_id: 10,
          exam_id: 1,
          actor_id: 501,
          actor_role: "teacher",
          space_id: 301,
          answer_version: 7,
          score: "4.5",
          comment: "要点完整",
        }),
        method: "POST",
      }),
    );
  });
});
