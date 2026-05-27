import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createResultsAPI } from "./results";

describe("results api", () => {
  test("读取成绩列表并映射后端字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 1,
            student_name: "张三",
            space_name: "高一 1 班",
            attempt_no: 1,
            objective_score: "2",
            subjective_score: "4.5",
            total_score: "6.5",
            submitted_at: 1779792000000,
            status: "可发布",
          }],
        },
      })),
    );
    const api = createResultsAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listResults({
      tenantID: 10,
      examID: 1,
      actorID: 501,
      actorRole: "teacher",
      spaceID: 301,
    })).resolves.toEqual({
      items: [{
        id: 1,
        studentName: "张三",
        spaceName: "高一 1 班",
        attemptNo: 1,
        objectiveScore: "2",
        subjectiveScore: "4.5",
        totalScore: "6.5",
        submittedAt: "2026-05-26 18:40",
        status: "可发布",
      }],
    });
  });

  test("保存发布配置并触发后端导出", async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 0, message: "ok", data: { saved: true } })))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: { file_path: "server/data/exports/exam-1-scores.csv", row_count: 1 },
      })));
    const api = createResultsAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.savePublishConfig({
      tenantID: 10,
      examID: 1,
      actorID: 501,
      actorRole: "teacher",
      spaceID: 301,
      publishMode: "manual_publish",
      scorePublishTime: 1779795600000,
    });
    await expect(api.exportResults({
      tenantID: 10,
      examID: 1,
      actorID: 501,
      actorRole: "teacher",
      spaceID: 301,
    })).resolves.toEqual({
      filePath: "server/data/exports/exam-1-scores.csv",
      rowCount: 1,
    });

    expect(fetcher).toHaveBeenNthCalledWith(
      1,
      "/api/v1/results/publish-config",
      expect.objectContaining({
        body: JSON.stringify({
          tenant_id: 10,
          exam_id: 1,
          actor_id: 501,
          actor_role: "teacher",
          space_id: 301,
          publish_mode: "manual_publish",
          score_publish_time: 1779795600000,
        }),
      }),
    );
  });
});
