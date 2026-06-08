import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createExamDetailAPI } from "./examDetail";

describe("examDetailApi", () => {
  test("按考试详情接口读取首屏、概览和试卷预览数据", async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);

      if (url === "/api/v1/exams/8/detail?tenant_id=10&space_id=301") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          targets: [{ target_type: "space", target_id: 301 }],
          target_space_ids: [301],
          allowed_space_ids: [301],
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/overview?tenant_id=10&space_id=301") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          candidate_stats: {
            planned: 2,
            joined: 1,
            submitted: 1,
            in_progress: 0,
          },
          question_types: [{
            question_type: "single",
            question_count: 1,
            total_score: "3",
          }],
          recent_activities: [{
            operation_type: "publish_exam",
            operation_title: "发布考试",
            operation_detail: "张老师 发布了考试",
            created_at: 1779792000000,
          }],
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/paper-preview?tenant_id=10&space_id=301&type=single&page=1&page_size=1") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          sections: [{
            section_id: 11,
            section_name: "一、单项选择题",
            question_type: "single",
            question_count: 1,
            total_score: "3",
          }],
          items: [{
            section_id: 11,
            section_name: "一、单项选择题",
            question_id: 201,
            question_type: "single",
            title: "后端返回的单选题",
            score: "3",
            blank_count: 0,
            sort_order: 1,
            options: [{ id: 1, key: "A", content: "选项 A" }],
          }],
          page: 1,
          page_size: 1,
          total: 1,
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/candidates?tenant_id=10&space_id=301&keyword=%E5%BC%A0&status=submitted&page=2&page_size=20") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          items: [{
            user_id: 21,
            username: "2024100101",
            real_name: "张三",
            space_id: 301,
            space_name: "高一1班",
            status: "submitted",
            started_at: 1779792000000,
            submitted_at: 1779795600000,
            total_score: "98",
            attempt_count: 2,
            current_attempt_id: null,
            result_attempt_id: 1801,
            source_targets: [{
              target_type: "space",
              target_id: 301,
              space_id: 301,
              space_name: "高一1班",
            }],
          }],
          page: 2,
          page_size: 20,
          total: 21,
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/candidates/import") {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(String(init?.body))).toEqual({
          tenant_id: 10,
          space_id: 301,
          user_ids: [21, 22],
        });
        return jsonResponse({
          imported_count: 1,
          skipped_count: 1,
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/invitations/resend") {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(String(init?.body))).toEqual({
          tenant_id: 10,
          space_id: 301,
          user_ids: [21],
        });
        return jsonResponse({
          sent_count: 1,
          skipped_count: 0,
          invite_code: "PM8888",
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/results/summary?tenant_id=10&space_id=301") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          stats: {
            submitted: 84,
            average_score: "86.5",
            highest_score: "118",
            pass_rate: "82%",
            pending_subjective: 16,
          },
          score_distribution: [{
            label: "80-89",
            count: 28,
          }],
          question_type_rates: [{
            question_type: "single",
            question_type_label: "单选题",
            average_rate: 92,
          }],
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/results?tenant_id=10&space_id=301&keyword=%E5%BC%A0&status=published&page=2&page_size=20") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          items: [{
            rank: 3,
            attempt_id: 1801,
            user_id: 21,
            username: "2024100101",
            real_name: "张三",
            space_id: 301,
            space_name: "高一1班",
            objective_score: "63",
            subjective_score: "48",
            total_score: "111",
            status: "published",
            submitted_at: 1779795600000,
          }],
          page: 2,
          page_size: 20,
          total: 84,
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/logs?tenant_id=10&space_id=301&operation_type=send_invite&page=1&page_size=20") {
        expect(init?.method).toBe("GET");
        return jsonResponse({
          exam: examResponse(),
          items: [{
            id: 3002,
            operation_type: "send_invite",
            operation_title: "重发邀请码",
            operation_detail: "重发 1 名考生邀请码",
            actor_id: 12,
            actor_type: "tenant_user",
            actor_role: "space_admin",
            operation_group_id: "seed-group-1",
            space_id: 301,
            created_at: 1779795600000,
          }],
          page: 1,
          page_size: 20,
          total: 1,
          permissions: permissionsResponse(),
        });
      }

      if (url === "/api/v1/exams/8/settings") {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(String(init?.body))).toEqual({
          tenant_id: 10,
          space_id: 301,
          publish_mode: "immediate_score",
          score_publish_time: null,
        });
        return jsonResponse({
          exam: {
            ...examResponse(),
            publish_mode: "immediate_score",
            score_publish_time: null,
          },
          permissions: permissionsResponse(),
        });
      }

      throw new Error(`unexpected request: ${url}`);
    });
    const api = createExamDetailAPI(createApiClient({ baseUrl: "", fetcher }));

    const detail = await api.getDetail({ tenantID: 10, examID: 8, spaceID: 301 });
    const overview = await api.getOverview({ tenantID: 10, examID: 8, spaceID: 301 });
    const preview = await api.getPaperPreview({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      questionType: "single",
      page: 1,
      pageSize: 1,
    });
    const candidates = await api.getCandidates({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      keyword: "张",
      status: "submitted",
      page: 2,
      pageSize: 20,
    });
    const imported = await api.importCandidates({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      userIDs: [21, 22],
    });
    const resent = await api.resendInvitations({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      userIDs: [21],
    });
    const resultSummary = await api.getResultsSummary({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
    });
    const results = await api.getResults({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      keyword: "张",
      status: "published",
      page: 2,
      pageSize: 20,
    });
    const logs = await api.getOperationLogs({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      operationType: "send_invite",
      page: 1,
      pageSize: 20,
    });
    const updatedSettings = await api.updateSettings({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      publishMode: "immediate_score",
      scorePublishTime: null,
    });

    expect(detail.exam.name).toBe("高一数学月考（2024-09）");
    expect(detail.permissions.canViewPaper).toBe(true);
    expect(overview.candidateStats.planned).toBe(2);
    expect(overview.questionTypes[0]).toEqual({ questionType: "single", questionCount: 1, totalScore: "3" });
    expect(preview.items[0].title).toBe("后端返回的单选题");
    expect(preview.items[0].options[0]).toEqual({ id: 1, key: "A", content: "选项 A" });
    expect(candidates.total).toBe(21);
    expect(candidates.items[0]).toMatchObject({
      userID: 21,
      username: "2024100101",
      realName: "张三",
      status: "submitted",
      attemptCount: 2,
      resultAttemptID: 1801,
    });
    expect(candidates.items[0].sourceTargets[0]).toEqual({
      targetType: "space",
      targetID: 301,
      spaceID: 301,
      spaceName: "高一1班",
    });
    expect(imported).toMatchObject({ importedCount: 1, skippedCount: 1 });
    expect(resent).toMatchObject({ sentCount: 1, skippedCount: 0, inviteCode: "PM8888" });
    expect(resultSummary.stats).toEqual({
      submitted: 84,
      averageScore: "86.5",
      highestScore: "118",
      passRate: "82%",
      pendingSubjective: 16,
    });
    expect(resultSummary.questionTypeRates[0]).toEqual({
      questionType: "single",
      questionTypeLabel: "单选题",
      averageRate: 92,
    });
    expect(results.items[0]).toMatchObject({
      rank: 3,
      attemptID: 1801,
      userID: 21,
      username: "2024100101",
      realName: "张三",
      objectiveScore: "63",
      subjectiveScore: "48",
      totalScore: "111",
      status: "published",
    });
    expect(logs.items[0]).toMatchObject({
      id: 3002,
      operationType: "send_invite",
      operationTitle: "重发邀请码",
      operationDetail: "重发 1 名考生邀请码",
      actorRole: "space_admin",
      operationGroupID: "seed-group-1",
      spaceID: 301,
      createdAt: 1779795600000,
    });
    expect(updatedSettings.exam.publishMode).toBe("immediate_score");
  });
});

function examResponse() {
  return {
    id: 8,
    tenant_id: 10,
    paper_id: 100,
    name: "高一数学月考（2024-09）",
    start_time: 1779792000000,
    end_time: 1779799200000,
    duration_minutes: 120,
    max_attempts: 1,
    result_strategy: "latest",
    publish_mode: "manual_publish",
    score_publish_time: null,
    invite_code: "PM8888",
    status: "published",
    target_type: "space",
    target_id: 301,
  };
}

function permissionsResponse() {
  return {
    can_view_detail: true,
    can_view_overview: true,
    can_view_paper: true,
    can_view_candidates: true,
    can_manage_candidates: true,
    can_view_results: true,
    can_export_results: true,
    can_publish_results: true,
    can_update_settings: true,
    can_view_logs: true,
  };
}

function jsonResponse(data: unknown) {
  return new Response(JSON.stringify({ code: 0, message: "ok", data }));
}
