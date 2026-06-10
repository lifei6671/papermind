import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createExamAPI } from "./exams";

describe("examApi", () => {
  test("考试列表请求携带分页参数并回填分页元数据", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("GET");
      expect(String(input)).toBe("/api/v1/exams?tenant_id=10&space_id=301&paper_id=100&page=2&page_size=50");
      return jsonResponse({
        items: [{
          id: 8,
          tenant_id: 10,
          paper_id: 100,
          name: "高一数学月考",
          start_time: 1779792000000,
          end_time: 1779799200000,
          duration_minutes: 120,
          invite_code: "PM8888",
          status: "published",
          created_by: 501,
          target_type: "space",
          target_id: 301,
          targets: [
            { target_type: "space", target_id: 301 },
            { target_type: "user", target_id: 501, scope_space_ids: [301] },
          ],
        }],
        page: 2,
        page_size: 50,
        total: 128,
      });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.listExams({
      tenantID: 10,
      spaceID: 301,
      paperID: 100,
      page: 2,
      pageSize: 50,
    });

    expect(result).toEqual({
      items: [{
        id: 8,
        tenantID: 10,
        paperID: 100,
        name: "高一数学月考",
        paperName: "试卷 100",
        inviteCode: "PM8888",
        target: "空间 301、用户 501",
        targets: [
          { targetType: "space", targetID: 301 },
          { targetType: "user", targetID: 501, scopeSpaceIDs: [301] },
        ],
        createdBy: 501,
        status: "published",
        startTime: 1779792000000,
        endTime: 1779799200000,
        startAt: "2026-05-26 18:40",
        endAt: "2026-05-26 20:40",
        durationMinutes: 120,
        maxAttempts: 0,
        resultStrategy: "latest",
        publishMode: "manual_publish",
        scorePublishTime: null,
      }],
      page: 2,
      pageSize: 50,
      total: 128,
    });
  });

  test("考试列表请求只在关键词非空时发送服务端搜索参数", async () => {
    const requestedURLs: string[] = [];
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      requestedURLs.push(String(input));
      return jsonResponse({
        items: [],
        page: 1,
        page_size: 5,
        total: 0,
      });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.listExams({ tenantID: 10, spaceID: 301, search: "  数学  ", page: 1, pageSize: 5 });
    await api.listExams({ tenantID: 10, spaceID: 301, search: "   ", page: 1, pageSize: 5 });

    expect(requestedURLs).toEqual([
      "/api/v1/exams?tenant_id=10&space_id=301&search=%E6%95%B0%E5%AD%A6&page=1&page_size=5",
      "/api/v1/exams?tenant_id=10&space_id=301&page=1&page_size=5",
    ]);
  });

  test("发布考试请求同时发送多目标数组和兼容单目标字段", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        paper_id: 100,
        name: "高一数学月考",
        target_type: "space",
        target_id: 301,
        targets: [
          { target_type: "space", target_id: 301 },
          { target_type: "user", target_id: 501, scope_space_ids: [301] },
        ],
        start_time: 1779792000000,
        end_time: 1779799200000,
        duration_minutes: 120,
        max_attempts: 2,
        result_strategy: "highest",
        publish_mode: "manual_publish",
        score_publish_time: 1779885600000,
        status: "published",
      });
      return jsonResponse({
        id: 8,
        tenant_id: 10,
        paper_id: 100,
        name: "高一数学月考",
        start_time: 1779792000000,
        end_time: 1779799200000,
        duration_minutes: 120,
        invite_code: "PM8888",
        status: "published",
        target_type: "space",
        target_id: 301,
        targets: [
          { target_type: "space", target_id: 301 },
          { target_type: "user", target_id: 501, scope_space_ids: [301] },
        ],
      });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.publishExam({
      tenantID: 10,
      paperID: 100,
      name: "高一数学月考",
      targets: [
        { targetType: "space", targetID: 301 },
        { targetType: "user", targetID: 501, scopeSpaceIDs: [301] },
      ],
      startTime: 1779792000000,
      endTime: 1779799200000,
      durationMinutes: 120,
      maxAttempts: 2,
      resultStrategy: "highest",
      publishMode: "manual_publish",
      scorePublishTime: 1779885600000,
      status: "published",
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/exams",
      expect.objectContaining({ method: "POST" }),
    );
    expect(result.target).toBe("空间 301、用户 501");
  });

  test("考试状态更新请求发送目标状态并回填考试", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("/api/v1/exams/8/status");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        status: "closed",
      });
      return jsonResponse({
        id: 8,
        tenant_id: 10,
        paper_id: 100,
        name: "高一数学月考",
        start_time: 1779792000000,
        end_time: 1779799200000,
        duration_minutes: 120,
        invite_code: "PM8888",
        status: "closed",
        target_type: "space",
        target_id: 301,
      });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.updateExamStatus!({
      tenantID: 10,
      examID: 8,
      status: "closed",
    });

    expect(result.status).toBe("closed");
  });

  test("草稿考试更新请求发送草稿配置和目标范围", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("/api/v1/exams/8/draft");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        paper_id: 100,
        name: "高一数学草稿",
        target_type: "space",
        target_id: 301,
        targets: [{ target_type: "space", target_id: 301 }],
        start_time: 1779792000000,
        end_time: 1779799200000,
        duration_minutes: 120,
        max_attempts: 1,
        result_strategy: "latest",
        publish_mode: "manual_publish",
        score_publish_time: null,
        status: "draft",
      });
      return jsonResponse({
        id: 8,
        tenant_id: 10,
        paper_id: 100,
        name: "高一数学草稿",
        start_time: 1779792000000,
        end_time: 1779799200000,
        duration_minutes: 120,
        max_attempts: 1,
        result_strategy: "latest",
        publish_mode: "manual_publish",
        score_publish_time: null,
        invite_code: "",
        status: "draft",
        target_type: "space",
        target_id: 301,
      });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.updateDraftExam!({
      tenantID: 10,
      examID: 8,
      paperID: 100,
      name: "高一数学草稿",
      targets: [{ targetType: "space", targetID: 301 }],
      startTime: 1779792000000,
      endTime: 1779799200000,
      durationMinutes: 120,
      maxAttempts: 1,
      resultStrategy: "latest",
      publishMode: "manual_publish",
      scorePublishTime: null,
      status: "draft",
    });

    expect(result.status).toBe("draft");
    expect(result.inviteCode).toBe("");
    expect(result.targets).toEqual([{ targetType: "space", targetID: 301 }]);
  });

  test("考试入口按邀请码解析考试信息", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        invite_code: "PM2026",
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
          created_by: 501,
        },
      }));
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const exam = await api.resolveInvite({ inviteCode: "PM2026" });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/exam-entry/invite/resolve",
      expect.objectContaining({ method: "POST" }),
    );
    expect(exam).toEqual({
      id: 1,
      tenantID: 10,
      paperID: 100,
      name: "高一语文期中考试",
      paperName: "试卷 100",
      inviteCode: "PM2026",
      target: "未配置",
      targets: [],
      createdBy: 501,
      status: "published",
      startTime: 1779792000000,
      endTime: 1779799200000,
      startAt: "2026-05-26 18:40",
      endAt: "2026-05-26 20:40",
      durationMinutes: 120,
      maxAttempts: 1,
      resultStrategy: "latest",
      publishMode: "manual_publish",
      scorePublishTime: null,
    });
  });

  test("答题 API 封装开始考试、保存答案和交卷请求", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/exam-entry/exams/1/attempts/start")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10 });
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
      if (url.endsWith("/api/v1/exam-entry/attempts/99/answers/9001")) {
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
      if (url.endsWith("/api/v1/exam-entry/attempts/99/submit")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          exam_token: "exam-token",
          event_type: "submit",
        });
        return jsonResponse({ submitted: true });
      }
      if (url.endsWith("/api/v1/exam-entry/results/99")) {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          attempt_id: 99,
          exam_id: 1,
          attempt_no: 1,
          objective_score: "6",
          subjective_score: "2",
          total_score: "8",
          analysis_visible: true,
        });
      }
      throw new Error(`unexpected request: ${url}`);
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const started = await api.startAttempt({ tenantID: 10, examID: 1 });
    await api.saveAnswer({
      tenantID: 10,
      attemptID: started.attemptID,
      attemptQuestionID: started.questions[0].id,
      examToken: started.examToken,
      questionType: "single",
      optionIDs: [101],
    });
    await api.submitAttempt({ tenantID: 10, attemptID: started.attemptID, examToken: started.examToken });
    const result = await api.getVisibleResult(started.attemptID);

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
    expect(result).toEqual({
      attemptID: 99,
      examID: 1,
      attemptNo: 1,
      objectiveScore: "6",
      subjectiveScore: "2",
      totalScore: "8",
      analysisVisible: true,
    });
  });

  test("开始考试和保存填空题答案时保留 blank_count 与多空 JSON 文本", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/exam-entry/exams/1/attempts/start")) {
        return jsonResponse({
          attempt: { id: 99, answer_deadline: 1779795600000 },
          exam_token: "exam-token",
          questions: [{
            id: 9002,
            sort_order: 2,
            section: { name: "三、填空题", instructions: "每题 2 分" },
            question: { title: "填写两个目录", type: "fill_blank", blank_count: 2 },
            options: [],
            score: "2",
          }],
        });
      }
      if (url.endsWith("/api/v1/exam-entry/attempts/99/answers/9002")) {
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          exam_token: "exam-token",
          question_type: "fill_blank",
          option_ids: [],
          text: "[\"/home\",\"/root\"]",
        });
        return jsonResponse({ saved: true, updated_at: 1779792000000 });
      }
      throw new Error(`unexpected request: ${url}`);
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    const started = await api.startAttempt({ tenantID: 10, examID: 1 });
    await api.saveAnswer({
      tenantID: 10,
      attemptID: 99,
      attemptQuestionID: 9002,
      examToken: "exam-token",
      questionType: "fill_blank",
      text: "[\"/home\",\"/root\"]",
    });

    expect(started.questions[0]).toEqual({
      id: 9002,
      number: 2,
      sectionTitle: "三、填空题",
      sectionSubtitle: "每题 2 分",
      type: "fill_blank",
      stem: "填写两个目录",
      score: 2,
      options: [],
      blankCount: 2,
    });
  });
  test("交卷请求支持自动交卷事件类型", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        exam_token: "exam-token",
        event_type: "auto_submit",
      });
      return jsonResponse({ submitted: true });
    });
    const api = createExamAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.submitAttempt({ tenantID: 10, attemptID: 99, examToken: "exam-token", eventType: "auto_submit" });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/exam-entry/attempts/99/submit",
      expect.objectContaining({ method: "POST" }),
    );
  });

});

function jsonResponse(data: unknown) {
  return new Response(JSON.stringify({ code: 0, message: "ok", data }));
}
