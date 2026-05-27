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
});
