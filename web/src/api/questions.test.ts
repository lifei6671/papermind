import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createQuestionAPI } from "./questions";

describe("questionApi", () => {
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
      errors: [{ rowNumber: 3, reason: "choice question needs correct answer" }],
    });
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
          status: "enabled",
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
  });
});
