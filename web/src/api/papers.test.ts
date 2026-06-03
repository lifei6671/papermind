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
            duration_minutes: 120,
            grade_level: "高一",
            total_score: "30",
            build_mode: "manual",
            shuffle_questions: false,
            show_analysis: true,
            status: "draft",
            created_at: 1717291800000,
            creator_name: "teacher.exam",
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
        durationMinutes: 120,
        gradeLevel: "高一",
        buildMode: "manual",
        status: "draft",
        totalScore: "30",
        createdAt: 1717291800000,
        creatorName: "teacher.exam",
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

  test("创建和删除试卷时调用试卷主资源接口", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/papers/100?tenant_id=10")) {
        expect(init?.method).toBe("DELETE");
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: { deleted: true },
        }));
      }
      expect(url).toBe("/api/v1/papers");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        name: "租户公共试卷",
        description: "公共资源",
        duration_minutes: 150,
        grade_level: "高一",
        shuffle_questions: true,
        show_analysis: true,
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 100,
          tenant_id: 10,
          space_id: null,
          name: "租户公共试卷",
          description: "公共资源",
          duration_minutes: 150,
          grade_level: "高一",
          total_score: "0",
          build_mode: "manual",
          shuffle_questions: true,
          show_analysis: true,
          status: "draft",
          created_at: 1717291800000,
          creator_name: "tenant.admin",
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createPaper({
      tenantID: 10,
      name: "租户公共试卷",
      description: "公共资源",
      durationMinutes: 150,
      gradeLevel: "高一",
      shuffleQuestions: true,
      showAnalysis: true,
    })).resolves.toEqual({
      id: 100,
      tenantID: 10,
      name: "租户公共试卷",
      description: "公共资源",
      durationMinutes: 150,
      gradeLevel: "高一",
      buildMode: "manual",
      status: "draft",
      totalScore: "0",
      createdAt: 1717291800000,
      creatorName: "tenant.admin",
    });
    await expect(api.deletePaper({ tenantID: 10, paperID: 100 })).resolves.toBeUndefined();
  });

  test("编辑、启用和禁用试卷时调用对应主资源接口", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/papers/100/enable")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10 });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: 100,
            tenant_id: 10,
            name: "高一语文月考试卷",
            description: "月考",
            duration_minutes: 120,
            total_score: "30",
            build_mode: "manual",
            shuffle_questions: false,
            show_analysis: true,
            status: "enabled",
            created_at: 1717291800000,
            creator_name: "teacher.exam",
          },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/disable")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({ tenant_id: 10 });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: 100,
            tenant_id: 10,
            name: "高一语文月考试卷",
            description: "月考",
            duration_minutes: 120,
            total_score: "30",
            build_mode: "manual",
            shuffle_questions: false,
            show_analysis: true,
            status: "disabled",
            created_at: 1717291800000,
            creator_name: "teacher.exam",
          },
        }));
      }
      expect(url).toBe("/api/v1/papers/100");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        name: "高一语文期末试卷",
        description: "文学阅读与语言基础",
        duration_minutes: 135,
        grade_level: "高二",
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 100,
          tenant_id: 10,
          name: "高一语文期末试卷",
          description: "文学阅读与语言基础",
          duration_minutes: 135,
          grade_level: "高二",
          total_score: "30",
          build_mode: "manual",
          shuffle_questions: false,
          show_analysis: true,
          status: "draft",
          created_at: 1717291800000,
          creator_name: "teacher.exam",
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.updatePaper({
      tenantID: 10,
      paperID: 100,
      name: "高一语文期末试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 135,
      gradeLevel: "高二",
    })).resolves.toEqual({
      id: 100,
      tenantID: 10,
      name: "高一语文期末试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 135,
      gradeLevel: "高二",
      buildMode: "manual",
      status: "draft",
      totalScore: "30",
      createdAt: 1717291800000,
      creatorName: "teacher.exam",
    });

    await expect(api.enablePaper({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      id: 100,
      tenantID: 10,
      name: "高一语文月考试卷",
      description: "月考",
      durationMinutes: 120,
      buildMode: "manual",
      status: "enabled",
      totalScore: "30",
      createdAt: 1717291800000,
      creatorName: "teacher.exam",
    });

    await expect(api.disablePaper({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      id: 100,
      tenantID: 10,
      name: "高一语文月考试卷",
      description: "月考",
      durationMinutes: 120,
      buildMode: "manual",
      status: "disabled",
      totalScore: "30",
      createdAt: 1717291800000,
      creatorName: "teacher.exam",
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

  test("试卷工作台接口支持切换模式、管理已选题和编辑规则", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/v1/papers/100/questions?tenant_id=10")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            items: [{
              tenant_id: 10,
              paper_id: 100,
              section_id: 1,
              question_id: 101,
              sort_order: 1,
              score: "6",
            }],
          },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/sections/1/questions/101/replace")) {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          new_question_id: 103,
          sort_order: 1,
          score: "8",
        });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            tenant_id: 10,
            paper_id: 100,
            section_id: 1,
            question_id: 103,
            sort_order: 1,
            score: "8",
          },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/sections/1/questions/101")) {
        if (init?.method === "PUT") {
          expect(JSON.parse(init?.body as string)).toEqual({
            tenant_id: 10,
            sort_order: 2,
            score: "7",
          });
          return new Response(JSON.stringify({
            code: 0,
            message: "ok",
            data: {
              tenant_id: 10,
              paper_id: 100,
              section_id: 1,
              question_id: 101,
              sort_order: 2,
              score: "7",
            },
          }));
        }
      }
      if (url.endsWith("/api/v1/papers/100/sections/1/questions/103?tenant_id=10")) {
        expect(init?.method).toBe("DELETE");
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: { deleted: true },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/rules/301?tenant_id=10")) {
        expect(init?.method).toBe("DELETE");
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: { deleted: true },
        }));
      }
      if (url.endsWith("/api/v1/papers/100/rules/301")) {
        expect(init?.method).toBe("PUT");
        expect(JSON.parse(init?.body as string)).toEqual({
          tenant_id: 10,
          section_id: 1,
          sort_order: 2,
          difficulty: "medium",
          tag_ids: [1],
          question_count: 3,
          score_per_question: "5",
        });
        return new Response(JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            id: 301,
            tenant_id: 10,
            paper_id: 100,
            section_id: 1,
            sort_order: 2,
            difficulty: "medium",
            tag_ids: [1],
            question_count: 3,
            score_per_question: "5",
          },
        }));
      }
      expect(url).toBe("/api/v1/papers/100/mode");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        build_mode: "rule_live",
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 100,
          tenant_id: 10,
          name: "高一语文月考试卷",
          description: "月考",
          total_score: "15",
          build_mode: "rule_live",
          shuffle_questions: false,
          show_analysis: true,
          status: "draft",
          created_at: 1717291800000,
          creator_name: "teacher.exam",
        },
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listSectionQuestions({ tenantID: 10, paperID: 100 })).resolves.toEqual({
      items: [{
        tenantID: 10,
        paperID: 100,
        sectionID: 1,
        questionID: 101,
        sortOrder: 1,
        score: "6",
      }],
    });
    await expect(api.updateSectionQuestion({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      sortOrder: 2,
      score: "7",
    })).resolves.toEqual({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      sortOrder: 2,
      score: "7",
    });
    await expect(api.replaceSectionQuestion({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      newQuestionID: 103,
      sortOrder: 1,
      score: "8",
    })).resolves.toEqual({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 103,
      sortOrder: 1,
      score: "8",
    });
    await expect(api.deleteSectionQuestion({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 103,
    })).resolves.toBeUndefined();
    await expect(api.updateRule({
      tenantID: 10,
      paperID: 100,
      ruleID: 301,
      sectionID: 1,
      sortOrder: 2,
      difficulty: "medium",
      tagIDs: [1],
      questionCount: 3,
      scorePerQuestion: "5",
    })).resolves.toEqual({
      id: 301,
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      sortOrder: 2,
      difficulty: "medium",
      tagIDs: [1],
      questionCount: 3,
      scorePerQuestion: "5",
    });
    await expect(api.deleteRule({ tenantID: 10, paperID: 100, ruleID: 301 })).resolves.toBeUndefined();
    await expect(api.updateBuildMode({ tenantID: 10, paperID: 100, buildMode: "rule_live" })).resolves.toEqual({
      id: 100,
      tenantID: 10,
      name: "高一语文月考试卷",
      description: "月考",
      durationMinutes: 120,
      buildMode: "rule_live",
      status: "draft",
      totalScore: "15",
      createdAt: 1717291800000,
      creatorName: "teacher.exam",
    });
  });

  test("大题重排接口会提交新的排序顺序", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("/api/v1/papers/100/sections/reorder");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({
        tenant_id: 10,
        orders: [
          { section_id: 12, sort_order: 1 },
          { section_id: 11, sort_order: 2 },
        ],
      });
      return new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: null,
      }));
    });
    const api = createPaperAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.reorderSections({
      tenantID: 10,
      paperID: 100,
      orders: [
        { sectionID: 12, sortOrder: 1 },
        { sectionID: 11, sortOrder: 2 },
      ],
    })).resolves.toBeUndefined();
  });
});
