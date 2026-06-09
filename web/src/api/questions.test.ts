import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createQuestionAPI } from "./questions";

describe("questionApi", () => {
  test("题库列表请求携带分页参数并返回分页信息", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe("/api/v1/questions?tenant_id=10&space_id=301&scope=public&page=2&page_size=30&search=%E5%87%BD%E6%95%B0&type=single&difficulty=medium&tag=%E5%87%BD%E6%95%B0&status=ready");

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 100,
              tenant_id: 10,
              space_id: 301,
              type: "single",
              difficulty: "medium",
              title: "题干",
              analysis: "解析",
              score_default: "2",
              status: "enabled",
              author_name: "teacher01",
              author_role: "teacher",
              created_at: 1700000000000,
              tag: "函数",
              tags: ["函数"],
              options: [
                { option_key: "A", content: "A", is_correct: true, is_distractor: false },
              ],
            },
          ],
          page: 2,
          page_size: 30,
          total: 87,
        },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.listQuestions({
      tenantID: 10,
      spaceID: 301,
      scope: "public",
      page: 2,
      pageSize: 30,
      search: " 函数 ",
      type: "single",
      difficulty: "medium",
      tag: " 函数 ",
      status: "ready",
    });

    expect(result.page).toBe(2);
    expect(result.pageSize).toBe(30);
    expect(result.total).toBe(87);
    expect(result.items).toHaveLength(1);
    expect(result.items[0].id).toBe(100);
    expect(result.items[0].spaceID).toBe(301);
  });

  test("题目标签候选使用后端范围过滤", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe("/api/v1/questions/tags?tenant_id=10&space_id=301&status=ready&search=%E5%87%BD");

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { items: ["函数", "函数压轴"] },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listQuestionTags({
      tenantID: 10,
      spaceID: 301,
      status: "ready",
      search: " 函 ",
    })).resolves.toEqual(["函数", "函数压轴"]);
  });

  test("题型可用数量使用后端聚合接口", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("/api/v1/questions/availability-counts");
      expect(init?.method).toBe("POST");
      expect(init?.body).toBe(JSON.stringify({
        tenant_id: 10,
        space_id: 301,
        status: "ready",
        tags: ["函数"],
        exclude_question_ids: [201],
      }));

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [
            { type: "single", count: 3 },
            { type: "judge", count: 1 },
          ],
        },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.countAvailableQuestions({
      tenantID: 10,
      spaceID: 301,
      status: "ready",
      tags: ["函数"],
      excludeQuestionIDs: [201],
    })).resolves.toEqual({ single: 3, judge: 1 });
  });

  test("导入题目时使用 multipart 表单提交文件和租户信息", async () => {
    const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      const body = init?.body as FormData;
      expect(body.get("tenant_id")).toBe("10");
      expect(body.get("file")).toBe(file);

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          success_count: 1,
          errors: [{ row_number: 3, reason: "choice question needs correct answer" }],
        },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.importQuestions({ tenantID: 10, file });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/questions/import",
      expect.objectContaining({ method: "POST" }),
    );
    expect(result).toEqual({
      successCount: 1,
      duplicateCount: 0,
      errors: [{ rowNumber: 3, reason: "choice question needs correct answer" }],
    });
  });

  test("启动异步导入任务时提交文件并返回任务 ID", async () => {
    const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      const body = init?.body as FormData;
      expect(body.get("tenant_id")).toBe("10");
      expect(body.get("space_id")).toBe("301");
      expect(body.get("status")).toBe("enabled");
      expect(body.get("file")).toBe(file);

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { job_id: "job-1" },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.startQuestionImportJob({ tenantID: 10, spaceID: 301, file, status: "enabled" });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/questions/import/jobs",
      expect.objectContaining({ method: "POST" }),
    );
    expect(result).toEqual({ jobID: "job-1" });
  });

  test("订阅异步导入任务时沿用 API baseUrl", () => {
    const createdSources: Array<{ url: string; init?: EventSourceInit }> = [];
    class FakeEventSource {
      onerror: ((event: Event) => void) | null = null;

      constructor(url: string | URL, init?: EventSourceInit) {
        createdSources.push({ url: String(url), init });
      }

      addEventListener() {
        return undefined;
      }

      close() {
        return undefined;
      }
    }
    vi.stubGlobal("EventSource", FakeEventSource);
    const api = createQuestionAPI(createApiClient({ baseUrl: "http://api.test", fetcher: vi.fn() }));

    const unsubscribe = api.subscribeQuestionImportJob({ jobID: "job-1" }, vi.fn(), vi.fn());
    unsubscribe();

    expect(createdSources).toEqual([{
      url: "http://api.test/api/v1/questions/import/jobs/job-1/events",
      init: { withCredentials: true },
    }]);
    vi.unstubAllGlobals();
  });

  test("创建题目时提交空间范围并使用响应字段", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(String(init?.body))).toEqual({
        tenant_id: 10,
        space_id: 301,
        type: "single",
        difficulty: "hard",
        title: "下列函数在 R 上单调递增的是哪一项？",
        analysis: "一次函数斜率为正时单调递增。",
        score_default: "6",
        tags: ["函数", "基础"],
        options: [
          { option_key: "A", content: "y = x", is_correct: true, is_distractor: false },
          { option_key: "B", content: "y = -x", is_correct: false, is_distractor: true },
        ],
      });

      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 101,
          tenant_id: 10,
          space_id: 301,
          type: "single",
          difficulty: "hard",
          title: "下列函数在 R 上单调递增的是哪一项？",
          analysis: "一次函数斜率为正时单调递增。",
          score_default: "6",
          status: "draft",
          author_name: "teacher01",
          author_role: "teacher",
          created_at: 1700000000000,
          tag: "函数",
          tags: ["函数", "基础"],
          options: [
            { option_key: "A", content: "y = x", is_correct: true, is_distractor: false },
            { option_key: "B", content: "y = -x", is_correct: false, is_distractor: true },
          ],
        },
      }));
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.createQuestion({
      tenantID: 10,
      spaceID: 301,
      type: "single",
      difficulty: "hard",
      title: "下列函数在 R 上单调递增的是哪一项？",
      options: ["y = x", "y = -x"],
      correctOptionIndexes: [0],
      analysis: "一次函数斜率为正时单调递增。",
      scoreDefault: "6",
      tags: ["函数", "基础"],
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/questions",
      expect.objectContaining({ method: "POST" }),
    );
    expect(result.title).toBe("下列函数在 R 上单调递增的是哪一项？");
    expect(result.stem).toBe("下列函数在 R 上单调递增的是哪一项？");
    expect(result.difficulty).toBe("hard");
    expect(result.tags).toEqual(["函数", "基础"]);
    expect(result.authorName).toBe("teacher01");
    expect(result.authorRole).toBe("teacher");
    expect(result.spaceID).toBe(301);
    expect(result.createdAt).toBe(1700000000000);
    expect(result.status).toBe("draft");
  });

  test("填空题创建会把多个标准答案编码成 JSON 数组，并在读取时解析", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(String(init?.body))).toEqual({
        tenant_id: 10,
        type: "fill_blank",
        difficulty: "medium",
        title: "填写两个路径",
        analysis: "两个空都要填写正确。",
        score_default: "3",
        standard_answer: "[\"/home\",\"/root\"]",
        blank_count: 2,
        tags: ["Linux"],
        options: [],
      });

      return questionResponse({
        type: "fill_blank",
        title: "填写两个路径",
        analysis: "两个空都要填写正确。",
        score_default: "3",
        standard_answer: "[\"/home\",\"/root\"]",
        options: [],
      });
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const result = await api.createQuestion({
      tenantID: 10,
      type: "fill_blank",
      difficulty: "medium",
      title: "填写两个路径",
      options: [],
      analysis: "两个空都要填写正确。",
      scoreDefault: "3",
      tags: ["Linux"],
      standardAnswer: "[\"/home\",\"/root\"]",
      blankCount: 2,
    });

    expect(result.standardAnswer).toBe("[\"/home\",\"/root\"]");
    expect(result.blankAnswers).toEqual(["/home", "/root"]);
  });

  test("更新、禁用和删除题目时提交租户和题目 ID", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/v1/questions/100" && init?.method === "PUT") {
        expect(JSON.parse(String(init.body))).toMatchObject({
          tenant_id: 10,
          space_id: null,
          title: "更新后的题干",
        });
        return questionResponse({ title: "更新后的题干" });
      }
      if (url === "/api/v1/questions/100/disable" && init?.method === "POST") {
        expect(JSON.parse(String(init.body))).toEqual({ tenant_id: 10 });
        return questionResponse({ status: "disabled" });
      }
      if (url === "/api/v1/questions/100" && init?.method === "DELETE") {
        expect(JSON.parse(String(init.body))).toEqual({ tenant_id: 10 });
        return new Response(JSON.stringify({ code: 0, message: "ok", data: null }));
      }
      throw new Error(`unexpected request ${init?.method} ${url}`);
    });
    const api = createQuestionAPI(createApiClient({ baseUrl: "", fetcher }));

    const updated = await api.updateQuestion({
      tenantID: 10,
      questionID: 100,
      spaceID: null,
      type: "single",
      difficulty: "medium",
      title: "更新后的题干",
      options: ["A", "B"],
      correctOptionIndexes: [0],
      analysis: "解析",
      scoreDefault: "2",
      tags: ["函数"],
    });
    const disabled = await api.disableQuestion({ tenantID: 10, questionID: 100 });
    await api.deleteQuestion({ tenantID: 10, questionID: 100 });

    expect(updated.title).toBe("更新后的题干");
    expect(disabled.status).toBe("disabled");
  });
});

function questionResponse(overrides: Record<string, unknown> = {}) {
  return new Response(JSON.stringify({
    code: 0,
    message: "ok",
    data: {
      id: 100,
      tenant_id: 10,
      type: "single",
      difficulty: "medium",
      title: "题干",
      analysis: "解析",
      score_default: "2",
      status: "enabled",
      author_name: "teacher01",
      author_role: "teacher",
      created_at: 1700000000000,
      tag: "函数",
      tags: ["函数"],
      options: [
        { option_key: "A", content: "A", is_correct: true, is_distractor: false },
        { option_key: "B", content: "B", is_correct: false, is_distractor: true },
      ],
      ...overrides,
    },
  }));
}
