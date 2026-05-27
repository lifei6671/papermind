import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createPaperAPI } from "./papers";

describe("paperApi", () => {
  test("读取试卷列表和大题结构时映射服务端字段", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes("/api/v1/papers/100/sections")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 200,
              tenant_id: 10,
              paper_id: 100,
              sort_order: 1,
              name: "一、现代文阅读",
              question_type: "single",
              instructions: "阅读材料后作答。",
              total_score: "30",
              question_count: 5,
            }],
          },
        }));
      }
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 100,
            tenant_id: 10,
            name: "高一语文月考试卷",
            description: "月考",
            total_score: "30",
            build_mode: "manual",
            shuffle_questions: false,
            show_analysis: true,
            status: "draft",
          }],
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listPapers({ tenantID: 10 })).resolves.toEqual({
      items: [{
        id: 100,
        tenantID: 10,
        name: "高一语文月考试卷",
        description: "月考",
        buildMode: "manual",
        status: "draft",
        totalScore: "30",
      }],
    });
    await expect(api.listSections({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      items: [{
        id: 200,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、现代文阅读",
        questionType: "single",
        instructions: "阅读材料后作答。",
        totalScore: "30",
        questionCount: 5,
      }],
    });
  });

  test("创建大题时提交试卷 ID 和大题信息", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        name: "二、语言文字运用",
        question_type: "single",
        instructions: "请从四个选项中选出最佳答案。",
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 201,
          tenant_id: 10,
          paper_id: 100,
          sort_order: 2,
          name: "二、语言文字运用",
          question_type: "single",
          instructions: "请从四个选项中选出最佳答案。",
          total_score: "0",
          question_count: 0,
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    const section = await api.createSection({
      tenantID: 10,
      paperID: 100,
      name: "二、语言文字运用",
      questionType: "single",
      instructions: "请从四个选项中选出最佳答案。",
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/papers/100/sections",
      expect.objectContaining({ method: "POST" }),
    );
    expect(section).toEqual({
      id: 201,
      tenantID: 10,
      paperID: 100,
      sortOrder: 2,
      name: "二、语言文字运用",
      questionType: "single",
      instructions: "请从四个选项中选出最佳答案。",
      totalScore: "0",
      questionCount: 0,
    });
  });

  test("手动选题时提交大题、题目和分值", async () => {
    const fetcher = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        question_id: 101,
        score: "6",
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          tenant_id: 10,
          paper_id: 100,
          section_id: 1,
          question_id: 101,
          sort_order: 1,
          score: "6",
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    const item = await api.addManualQuestion({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      score: "6",
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/papers/100/sections/1/questions",
      expect.objectContaining({ method: "POST" }),
    );
    expect(item).toEqual({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      sortOrder: 1,
      score: "6",
    });
  });

  test("组卷规则接口映射规则字段、生成和预检查结果", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/papers/100/rules?tenant_id=10")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 301,
              tenant_id: 10,
              paper_id: 100,
              section_id: 1,
              sort_order: 1,
              difficulty: "easy",
              tag_ids: [1],
              question_count: 6,
              score_per_question: "4",
              shuffle_options: true,
            }],
          },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/rule-fixed/generate")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10 });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: { paper_id: 100, generated: true },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/rule-live/precheck")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10 });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: { candidate_question_ids: [101, 102], candidate_count: 2 },
        }));
      }

      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        sort_order: 1,
        difficulty: "easy",
        tag_ids: [1],
        question_count: 6,
        score_per_question: "4",
        shuffle_options: true,
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 301,
          tenant_id: 10,
          paper_id: 100,
          section_id: 1,
          sort_order: 1,
          difficulty: "easy",
          tag_ids: [1],
          question_count: 6,
          score_per_question: "4",
          shuffle_options: true,
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createRule({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      sortOrder: 1,
      difficulty: "easy",
      tagIDs: [1],
      questionCount: 6,
      scorePerQuestion: "4",
      shuffleOptions: true,
    })).resolves.toEqual({
      id: 301,
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      sortOrder: 1,
      difficulty: "easy",
      tagIDs: [1],
      questionCount: 6,
      scorePerQuestion: "4",
      shuffleOptions: true,
    });
    await expect(api.listRules({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      items: [{
        id: 301,
        tenantID: 10,
        paperID: 100,
        sectionID: 1,
        sortOrder: 1,
        difficulty: "easy",
        tagIDs: [1],
        questionCount: 6,
        scorePerQuestion: "4",
        shuffleOptions: true,
      }],
    });
    await expect(api.generateRuleFixed({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      paperID: 100,
      generated: true,
    });
    await expect(api.precheckRuleLive({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      candidateQuestionIDs: [101, 102],
      candidateCount: 2,
    });
  });
});
