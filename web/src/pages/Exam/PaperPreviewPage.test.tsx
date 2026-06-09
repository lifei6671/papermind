/// <reference types="node" />

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { ApiError } from "../../api/client";
import type { ExamDetailAPI, ExamDetailExam, ExamDetailPermissions } from "../../api/examDetail";
import { SessionContext } from "../../auth/session-context";
import type { PaperAPI, PaperRow, PaperSectionRow, ManualQuestionRow } from "../../api/papers";
import type { QuestionAPI, QuestionRow } from "../../api/questions";
import { FeedbackProvider } from "../../app/feedback";
import { PaperPreviewPage } from "./PaperPreviewPage";

const { exportResultsMock } = vi.hoisted(() => ({
  exportResultsMock: vi.fn(async () => ({
    filePath: "/tmp/papermind-results.csv",
    fileURL: "https://example.com/papermind-results.csv",
    rowCount: 1,
  })),
}));

vi.mock("../../api/results", async () => {
  const actual = await vi.importActual<typeof import("../../api/results")>("../../api/results");
  return {
    ...actual,
    resultsApi: {
      listResults: vi.fn(async () => ({ items: [] })),
      savePublishConfig: vi.fn(async () => undefined),
      exportResults: exportResultsMock,
    },
  };
});

function expectSelectText(element: HTMLElement, text: string) {
  expect(element.closest(".ui-select-trigger")).toHaveTextContent(text);
}

test("试卷统计区在窄视口下有媒体查询兜底避免右侧裁切", () => {
  const css = readFileSync(join(process.cwd(), "src/styles/global.css"), "utf8");
  const summaryCardRule = css.match(/\.paper-preview-summary-card\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const summaryCardBodyRule = css.match(/\.paper-preview-summary-card\s+\.app-panel__body\s*\{[\s\S]*?\}/)?.[0] ?? "";

  // 统计项必须先按卡片真实宽度自动换行，不能默认固定 6 列，否则后台窄内容区会裁掉右侧指标。
  expect(css).toMatch(/\.paper-preview-summary\s*\{[\s\S]*?grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(184px,\s*100%\),\s*1fr\)\)/);
  // 统计卡片和 body 都不能用 hidden 裁掉溢出内容；真实断点未命中时也必须至少允许内容完整进入布局。
  expect(summaryCardRule).not.toMatch(/overflow:\s*hidden/);
  expect(summaryCardBodyRule).not.toMatch(/overflow:\s*hidden/);
  // 容器查询依赖父级尺寸计算，媒体查询兜底也必须保留自适应列宽，不能把统计区重新固定成会裁切的单行。
  expect(css).toMatch(/@media\s*\(max-width:\s*1280px\)[\s\S]*?\.paper-preview-summary\s*\{[\s\S]*?grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/);
  // 预览 tab 宽屏可保持参考图的六列视觉，但窄后台视口必须切成两行三列，避免中文长指标被压缩遮挡。
  expect(css).toMatch(/\.paper-preview-tab-panel\s*>\s*\.paper-preview-summary-card\s+\.paper-preview-summary\s*\{[\s\S]*?grid-template-columns:\s*repeat\(6,\s*minmax\(0,\s*1fr\)\)/);
  expect(css).toMatch(/@container\s*\(max-width:\s*1280px\)\s*\{[\s\S]*?\.paper-preview-tab-panel\s*>\s*\.paper-preview-summary-card\s+\.paper-preview-summary\s*\{[\s\S]*?grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/);
  expect(css).toMatch(/@media\s*\(max-width:\s*1280px\)[\s\S]*?\.paper-preview-tab-panel\s*>\s*\.paper-preview-summary-card\s+\.paper-preview-summary\s*\{[\s\S]*?grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/);
});

test("试卷预览页按考试预览布局展示试卷结构和题目", async () => {
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewPage
                paperApi={createPaperApiDouble()}
                questionApi={createQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "高一数学月考（2024-09）" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "返回试卷列表" })).toHaveAttribute("href", "/papers?space_id=301");
  expect(screen.getByText("已发布")).toHaveClass("status-badge--success");
  expect(screen.queryByRole("button", { name: "试卷详情" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "导出试卷" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "进入考试" })).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "试卷预览" })).toHaveAttribute("aria-selected", "true");

  const summary = screen.getByRole("region", { name: "试卷统计" });
  expect(within(summary).getByText("总题数")).toBeInTheDocument();
  expect(within(summary).getByText("33")).toBeInTheDocument();
  expect(within(summary).getByText("总分值")).toBeInTheDocument();
  expect(within(summary).getAllByText("120")).toHaveLength(2);
  expect(within(summary).getByText("考试时长")).toBeInTheDocument();
  expect(within(summary).getByText("合格分数")).toBeInTheDocument();
  expect(within(summary).getByText("72")).toBeInTheDocument();
  expect(within(summary).getByText("正式考试")).toBeInTheDocument();
  expect(within(summary).getByText("答题后公布")).toBeInTheDocument();

  const structure = screen.getByRole("navigation", { name: "试卷结构" });
  expect(within(structure).getByText("一、选择题（共20题，60分）")).toBeInTheDocument();
  expect(within(structure).getByText("单选题（共15题，45分）")).toHaveAttribute("aria-current", "true");
  expect(within(structure).getByText("多选题（共5题，15分）")).toBeInTheDocument();
  expect(within(structure).getByText("二、填空题（共5题，20分）")).toBeInTheDocument();
  expect(within(structure).getByText("三、解答题（共5题，30分）")).toBeInTheDocument();
  expect(within(structure).getByText("四、判断题（共3题，10分）")).toBeInTheDocument();

  expect(screen.getByRole("button", { name: "全部" })).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByRole("button", { name: "单选题" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "多选题" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "判断题" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "填空题" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "解答题" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "应用题" })).not.toBeInTheDocument();
  expectSelectText(screen.getByLabelText("每页题数"), "1");
  expect(screen.getByText("1 / 33 题")).toBeInTheDocument();

  const firstQuestion = screen.getByRole("article", { name: "第 1 题" });
  expect(within(firstQuestion).getByText("单选题")).toBeInTheDocument();
  expect(within(firstQuestion).getByText("1 / 33")).toBeInTheDocument();
  expect(within(firstQuestion).getByText("[分值 3分]")).toBeInTheDocument();
  expect(within(firstQuestion).getByText("已知集合 A = {x | -2 ≤ x ≤ 4}，B = {x | x > 1}，则 A∩B =（ ）")).toBeInTheDocument();
  expect(within(firstQuestion).getByText("{x | -2 ≤ x < 1}")).toHaveClass("paper-preview-question-card__option-text");
  expect(within(firstQuestion).getByText("{x | 1 < x < 4}")).toBeInTheDocument();
  expect(screen.queryByRole("article", { name: "第 2 题" })).not.toBeInTheDocument();
});

test("试卷预览页的每页题数和上下题控件会切换当前展示题目", async () => {
  const user = userEvent.setup();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewPage
                paperApi={createPaperApiDouble()}
                questionApi={createQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("article", { name: "第 1 题" })).toBeInTheDocument();
  expect(screen.queryByRole("article", { name: "第 2 题" })).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "下一题" }));

  expect(await screen.findByRole("article", { name: "第 2 题" })).toBeInTheDocument();
  expect(screen.queryByRole("article", { name: "第 1 题" })).not.toBeInTheDocument();
  expect(screen.getByText("2 / 33 题")).toBeInTheDocument();

  await user.click(screen.getByLabelText("每页题数"));
  await user.click(await screen.findByRole("option", { name: "5" }));

  expect(await screen.findByRole("article", { name: "第 1 题" })).toBeInTheDocument();
  expect(screen.getByRole("article", { name: "第 2 题" })).toBeInTheDocument();
  expect(screen.getByText("1 / 33 题")).toBeInTheDocument();
});

test("考试详情 canonical 路由使用考试详情接口渲染基础数据", async () => {
  const paperApi = createUnusedPaperApiDouble();
  const questionApi = createUnusedQuestionApiDouble();
  const examDetailApi = createExamDetailApiDouble();
  const user = userEvent.setup();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={paperApi}
                questionApi={questionApi}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  expect(screen.getByText("已发布")).toBeInTheDocument();

  const summary = screen.getByRole("region", { name: "试卷统计" });
  expect(within(summary).getByText("总题数")).toBeInTheDocument();
  expect(within(summary).getByText("1")).toBeInTheDocument();
  expect(within(summary).getByText("总分值")).toBeInTheDocument();
  expect(within(summary).getByText("3")).toBeInTheDocument();

  const structure = screen.getByRole("navigation", { name: "试卷结构" });
  expect(within(structure).getByText("一、单项选择题（共1题，3分）")).toBeInTheDocument();

  const question = screen.getByRole("article", { name: "第 1 题" });
  expect(within(question).getByText("后端返回题干")).toBeInTheDocument();
  expect(within(question).getByText("后端选项 A")).toBeInTheDocument();
  expect(screen.getByText("1 / 1 题")).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "考试概览" }));
  const overviewPanel = screen.getByRole("tabpanel", { name: "考试概览" });
  expect(within(overviewPanel).getByText("暂无近期动态")).toBeInTheDocument();
  expect(within(overviewPanel).queryByText("发布考试")).not.toBeInTheDocument();
  expect(within(overviewPanel).queryByText("考生开始作答")).not.toBeInTheDocument();

  expect(examDetailApi.getDetail).toHaveBeenCalledWith({ tenantID: 10, examID: 8, spaceID: 301 });
  expect(examDetailApi.getOverview).toHaveBeenCalledWith({ tenantID: 10, examID: 8, spaceID: 301 });
  expect(examDetailApi.getPaperPreview).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    questionType: undefined,
    page: 1,
    pageSize: 1,
  });
  expect(paperApi.listPapers).not.toHaveBeenCalled();
  expect(questionApi.listQuestions).not.toHaveBeenCalled();
});

test("考试详情 canonical 路由按 UI 分页请求试卷预览", async () => {
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getPaperPreview: vi.fn(async (input) => {
      if (input.page === 1 && input.pageSize === 1) {
        return {
          exam: examDetailExam(),
          sections: [{
            sectionID: 11,
            sectionName: "一、单项选择题",
            questionType: "single",
            questionCount: 2,
            totalScore: "6",
          }],
          items: [
            {
              sectionID: 11,
              sectionName: "一、单项选择题",
              questionID: 201,
              questionType: "single",
              title: "第一页题干",
              score: "3",
              blankCount: 0,
              sortOrder: 1,
              options: [{ id: 1, key: "A", content: "第一页选项" }],
            },
          ],
          page: 1,
          pageSize: input.pageSize,
          total: 2,
          permissions: examDetailPermissions(),
        };
      }
      if (input.page === 2 && input.pageSize === 1) {
        return {
          exam: examDetailExam(),
          sections: [{
            sectionID: 11,
            sectionName: "一、单项选择题",
            questionType: "single",
            questionCount: 2,
            totalScore: "6",
          }],
          items: [
            {
              sectionID: 11,
              sectionName: "一、单项选择题",
              questionID: 202,
              questionType: "single",
              title: "第二页题干",
              score: "3",
              blankCount: 0,
              sortOrder: 2,
              options: [{ id: 2, key: "A", content: "第二页选项" }],
            },
          ],
          page: 1,
          pageSize: input.pageSize,
          total: 2,
          permissions: examDetailPermissions(),
        };
      }
      throw new Error(`unexpected preview request ${JSON.stringify(input)}`);
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  const user = userEvent.setup();
  expect(await screen.findByText("第一页题干")).toBeInTheDocument();
  expect(screen.queryByText("第二页题干")).not.toBeInTheDocument();
  expect(screen.getByText("1 / 2 题")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "下一题" }));
  expect(await screen.findByText("第二页题干")).toBeInTheDocument();
  expect(screen.getByText("2 / 2 题")).toBeInTheDocument();
  expect(examDetailApi.getPaperPreview).toHaveBeenCalledTimes(2);
  expect(examDetailApi.getPaperPreview).toHaveBeenNthCalledWith(1, {
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    questionType: undefined,
    page: 1,
    pageSize: 1,
  });
  expect(examDetailApi.getPaperPreview).toHaveBeenNthCalledWith(2, {
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    questionType: undefined,
    page: 2,
    pageSize: 1,
  });
});

test("考试详情题型筛选后按后端筛选总数限制分页", async () => {
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getPaperPreview: vi.fn(async (input) => {
      const sections = [
        {
          sectionID: 11,
          sectionName: "一、单项选择题",
          questionType: "single",
          questionCount: 5,
          totalScore: "15",
        },
        {
          sectionID: 12,
          sectionName: "二、填空题",
          questionType: "fill_blank",
          questionCount: 1,
          totalScore: "3",
        },
      ];
      if (input.questionType === "fill_blank") {
        return {
          exam: examDetailExam(),
          sections,
          items: [
            {
              sectionID: 12,
              sectionName: "二、填空题",
              questionID: 206,
              questionType: "fill_blank",
              title: "筛选后的填空题",
              score: "3",
              blankCount: 1,
              sortOrder: 1,
              options: [],
            },
          ],
          page: 1,
          pageSize: input.pageSize,
          total: 1,
          permissions: examDetailPermissions(),
        };
      }
      return {
        exam: examDetailExam(),
        sections,
        items: [
          {
            sectionID: 11,
            sectionName: "一、单项选择题",
            questionID: 201,
            questionType: "single",
            title: "未筛选的单选题",
            score: "3",
            blankCount: 0,
            sortOrder: 1,
            options: [{ id: 1, key: "A", content: "未筛选选项" }],
          },
        ],
        page: 1,
        pageSize: input.pageSize,
        total: 6,
        permissions: examDetailPermissions(),
      };
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  const user = userEvent.setup();
  expect(await screen.findByText("未筛选的单选题")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "填空题" }));

  expect(await screen.findByText("筛选后的填空题")).toBeInTheDocument();
  expect(screen.getByText("1 / 1 题")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "下一题" })).toBeDisabled();
  expect(examDetailApi.getPaperPreview).toHaveBeenLastCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    questionType: "fill_blank",
    page: 1,
    pageSize: 1,
  });
});

test("考试详情子接口失败时保留已加载的基础信息", async () => {
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getPaperPreview: vi.fn(async () => {
      throw new ApiError("无权查看试卷预览", 403, 403, {});
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "考试概览" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "基本信息" })).toBeInTheDocument();
  expect(screen.queryByText("试卷预览加载失败")).not.toBeInTheDocument();
});

test("考试详情概览接口失败时不回退演示数据", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getOverview: vi.fn(async () => {
      throw new ApiError("考试概览加载失败", 500, 500, {});
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "考试概览" }));

  const overviewPanel = screen.getByRole("tabpanel", { name: "考试概览" });
  expect(within(overviewPanel).getByText("暂无近期动态")).toBeInTheDocument();
  const distribution = within(overviewPanel).getByRole("table", { name: "题型分布" });
  expect(distribution).toHaveTextContent("单选题");
  expect(distribution).toHaveTextContent("1");
  expect(distribution).toHaveTextContent("3");
  expect(distribution).not.toHaveTextContent("15");
  expect(distribution).not.toHaveTextContent("45");
  expect(within(overviewPanel).getAllByText("0").length).toBeGreaterThan(0);
  expect(within(overviewPanel).queryByText("张老师 发布了考试")).not.toBeInTheDocument();
  expect(within(overviewPanel).queryByText("已有 96 名考生进入考试并开始作答")).not.toBeInTheDocument();
  expect(within(overviewPanel).queryByText("12 名考生尚未开考")).not.toBeInTheDocument();
  expect(within(overviewPanel).queryByText("高一全年级")).not.toBeInTheDocument();
});

test("考试详情真实概览风险行不展示无行为详情按钮", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getOverview: vi.fn(async () => ({
      exam: examDetailExam(),
      candidateStats: {
        planned: 3,
        joined: 2,
        submitted: 1,
        inProgress: 1,
      },
      questionTypes: [{ questionType: "single", questionCount: 1, totalScore: "3" }],
      recentActivities: [],
      permissions: examDetailPermissions(),
    })),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "考试概览" }));

  const overviewPanel = screen.getByRole("tabpanel", { name: "考试概览" });
  expect(within(overviewPanel).getByText("1 名考生尚未开考")).toBeInTheDocument();
  expect(within(overviewPanel).getByText("1 名考生正在作答")).toBeInTheDocument();
  expect(within(overviewPanel).queryByRole("button", { name: "查看详情" })).not.toBeInTheDocument();
});

test("考试详情标签页按后端权限裁剪且不触发未授权数据请求", async () => {
  const readonlyPermissions: ExamDetailPermissions = {
    canViewDetail: true,
    canViewOverview: true,
    canViewPaper: false,
    canViewCandidates: false,
    canManageCandidates: false,
    canViewResults: false,
    canExportResults: false,
    canPublishResults: false,
    canUpdateSettings: false,
    canViewLogs: false,
  };
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getDetail: vi.fn(async () => ({
      exam: examDetailExam(),
      targets: [{ targetType: "space" as const, targetID: 301 }],
      targetSpaceIDs: [301],
      allowedSpaceIDs: [301],
      permissions: readonlyPermissions,
    })),
    getOverview: vi.fn(async () => ({
      exam: examDetailExam(),
      candidateStats: {
        planned: 1,
        joined: 0,
        submitted: 0,
        inProgress: 0,
      },
      questionTypes: [{ questionType: "single", questionCount: 1, totalScore: "3" }],
      recentActivities: [],
      permissions: readonlyPermissions,
    })),
    getPaperPreview: vi.fn(async () => {
      throw new ApiError("无权查看试卷预览", 403, 403, {});
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "考试概览" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "基本信息" })).toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "试卷预览" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "考生管理" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "成绩管理" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "考试设置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "操作日志" })).not.toBeInTheDocument();
  expect(examDetailApi.getPaperPreview).not.toHaveBeenCalled();
  expect(examDetailApi.getCandidates).not.toHaveBeenCalled();
  expect(examDetailApi.getResultsSummary).not.toHaveBeenCalled();
  expect(examDetailApi.getResults).not.toHaveBeenCalled();
  expect(examDetailApi.getOperationLogs).not.toHaveBeenCalled();
});

test("考试详情无权限时展示 toast 和无权限提示面板", async () => {
  const examDetailApi = createRejectedExamDetailApiDouble(new ApiError("没有权限查看该考试", 403, 403, {}));

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "没有权限查看该考试" })).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getAllByText("没有权限查看该考试").some((node) => node.closest(".ant-message") !== null)).toBe(true);
  });
  expect(screen.getByText("请确认当前账号是否拥有该考试所在空间的管理权限。")).toBeInTheDocument();
  expect(screen.queryByRole("region", { name: "试卷统计" })).not.toBeInTheDocument();
});

test("考试详情不存在时展示 404 提示面板", async () => {
  const examDetailApi = createRejectedExamDetailApiDouble(new ApiError("考试不存在或已删除", 404, 404, {}));

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/404?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "考试不存在或已删除" })).toBeInTheDocument();
  expect(screen.getByText("该考试可能已被删除，或当前空间下不存在这场考试。")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "返回考试列表" })).toHaveAttribute("href", "/exams?space_id=301");
  expect(screen.queryByRole("region", { name: "试卷统计" })).not.toBeInTheDocument();
});

test("考试管理菜单支持切换占位标签页", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  const questionApi = createQuestionApiDouble();
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewPage
                paperApi={paperApi}
                questionApi={questionApi}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("tab", { name: "试卷预览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tabpanel", { name: "试卷预览" })).toBeInTheDocument();
  expect(paperApi.listPapers).toHaveBeenCalledTimes(1);
  expect(paperApi.listSections).toHaveBeenCalledTimes(1);
  expect(paperApi.listSectionQuestions).toHaveBeenCalledTimes(1);
  expect(questionApi.listQuestions).toHaveBeenCalledTimes(1);

  await user.click(screen.getByRole("tab", { name: "考试监控" }));

  expect(screen.getByRole("tab", { name: "考试监控" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "试卷预览" })).toHaveAttribute("aria-selected", "false");
  expect(screen.getByRole("tabpanel", { name: "考试监控" })).toHaveTextContent("考试监控");
  expect(screen.getByRole("tabpanel", { name: "考试监控" })).toHaveTextContent("该模块内容正在接入，当前先保留入口位置。");
  expect(screen.queryByRole("navigation", { name: "试卷结构" })).not.toBeInTheDocument();
  expect(paperApi.listPapers).toHaveBeenCalledTimes(1);
  expect(paperApi.listSections).toHaveBeenCalledTimes(1);
  expect(paperApi.listSectionQuestions).toHaveBeenCalledTimes(1);
  expect(questionApi.listQuestions).toHaveBeenCalledTimes(1);

  await user.click(screen.getByRole("tab", { name: "试卷预览" }));

  expect(screen.getByRole("tab", { name: "试卷预览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tabpanel", { name: "试卷预览" })).toBeInTheDocument();
  expect(screen.getByRole("navigation", { name: "试卷结构" })).toBeInTheDocument();
});

test("考试概览标签页展示考试运营概览", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewPage
                paperApi={createPaperApiDouble()}
                questionApi={createQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "高一数学月考（2024-09）" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考试概览" }));

  expect(screen.getByRole("tab", { name: "考试概览" })).toHaveAttribute("aria-selected", "true");
  const panel = screen.getByRole("tabpanel", { name: "考试概览" });

  expect(within(panel).getByText("计划考生")).toBeInTheDocument();
  expect(within(panel).getByText("128")).toBeInTheDocument();
  expect(within(panel).getByText("已参加")).toBeInTheDocument();
  expect(within(panel).getByText("96")).toBeInTheDocument();
  expect(within(panel).getByText("已交卷")).toBeInTheDocument();
  expect(within(panel).getByText("84")).toBeInTheDocument();
  expect(within(panel).getByText("满分 / 及格")).toBeInTheDocument();
  expect(within(panel).getByText("120 / 72")).toBeInTheDocument();

  expect(within(panel).getByText("考试进度")).toBeInTheDocument();
  expect(within(panel).getByText("已完成率")).toBeInTheDocument();
  expect(within(panel).getByText("65")).toBeInTheDocument();
  expect(within(panel).getByText("未开始")).toBeInTheDocument();
  expect(within(panel).getByText("32 人")).toBeInTheDocument();

  const distribution = within(panel).getByRole("table", { name: "题型分布" });
  expect(distribution).toHaveTextContent("单选题");
  expect(distribution).toHaveTextContent("15");
  expect(distribution).toHaveTextContent("42.9%");
  expect(distribution).toHaveTextContent("合计");
  expect(distribution).toHaveTextContent("33");
  expect(distribution).toHaveTextContent("120");

  expect(within(panel).getByText("考试安排")).toBeInTheDocument();
  expect(within(panel).getByText("2024-09-28 09:00 - 11:00")).toBeInTheDocument();
  expect(within(panel).getByText("高一全年级")).toBeInTheDocument();
  expect(within(panel).getByText("风险提醒 / 监考提示")).toBeInTheDocument();
  expect(within(panel).getByText("12 名考生尚未开考")).toBeInTheDocument();
  expect(within(panel).getByText("2 名考生网络波动")).toBeInTheDocument();
  expect(within(panel).getByText("近期动态")).toBeInTheDocument();
  expect(within(panel).getByText("发布考试")).toBeInTheDocument();
  expect(within(panel).getByText("考生开始作答")).toBeInTheDocument();
});

test("基本信息标签页展示考试配置详情", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={createExamDetailApiDouble()}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "基本信息" }));

  expect(screen.getByRole("tab", { name: "基本信息" })).toHaveAttribute("aria-selected", "true");
  const panel = screen.getByRole("tabpanel", { name: "基本信息" });

  const baseInfo = within(panel).getByRole("region", { name: "考试基础信息" });
  expect(within(baseInfo).getByText("考试名称：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("后端考试详情")).toBeInTheDocument();
  expect(within(baseInfo).getByText("关联试卷：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("试卷 #100")).toBeInTheDocument();
  expect(within(baseInfo).getByText("考试状态：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("已发布")).toBeInTheDocument();
  expect(within(baseInfo).getByText("最大作答次数：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("1 次")).toBeInTheDocument();
  expect(within(baseInfo).getByText("成绩策略：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("按最新成绩")).toBeInTheDocument();
  expect(within(baseInfo).getByText("邀请码：")).toBeInTheDocument();
  expect(within(baseInfo).getByText("PM8888")).toBeInTheDocument();

  const schedule = within(panel).getByRole("region", { name: "时间与发布安排" });
  expect(within(schedule).getByText("考试开始时间：")).toBeInTheDocument();
  expect(within(schedule).getByText("考试结束时间：")).toBeInTheDocument();
  expect(within(schedule).getByText("发布范围：")).toBeInTheDocument();
  expect(within(schedule).getByText("空间 301")).toBeInTheDocument();
  expect(within(schedule).getByText("成绩公布方式：")).toBeInTheDocument();
  expect(within(schedule).getByText("手动发布成绩")).toBeInTheDocument();
  expect(within(schedule).getByText("总分：")).toBeInTheDocument();
  expect(within(schedule).getByText("3 分")).toBeInTheDocument();
  expect(within(schedule).getByText("及格分：")).toBeInTheDocument();
  expect(within(schedule).getByText("2 分")).toBeInTheDocument();
});

test("考生管理标签页展示考生列表和批量操作", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewPage
                paperApi={createPaperApiDouble()}
                questionApi={createQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "高一数学月考（2024-09）" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  expect(screen.getByRole("tab", { name: "考生管理" })).toHaveAttribute("aria-selected", "true");
  const panel = screen.getByRole("tabpanel", { name: "考生管理" });
  const stats = within(panel).getByRole("region", { name: "考生统计" });

  expect(within(stats).getByText("应考人数")).toBeInTheDocument();
  expect(within(stats).getByText("128")).toBeInTheDocument();
  expect(within(stats).getByText("未开始")).toBeInTheDocument();
  expect(within(stats).getByText("32")).toBeInTheDocument();
  expect(within(stats).getByText("进行中")).toBeInTheDocument();
  expect(within(stats).getByText("12")).toBeInTheDocument();
  expect(within(stats).getByText("已交卷")).toBeInTheDocument();
  expect(within(stats).getByText("84")).toBeInTheDocument();

  expect(within(panel).getByRole("button", { name: "批量导入考生" })).toBeInTheDocument();
  expect(within(panel).getByRole("button", { name: "发送邀请码" })).toBeInTheDocument();
  expect(within(panel).getByPlaceholderText("搜索姓名、学号或班级")).toBeInTheDocument();
  expectSelectText(within(panel).getByLabelText("考试状态筛选"), "全部状态");
  expectSelectText(within(panel).getByLabelText("班级筛选"), "高一全年级");

  const table = within(panel).getByRole("table", { name: "考生列表" });
  expect(table).toHaveTextContent("姓名");
  expect(table).toHaveTextContent("李明");
  expect(table).toHaveTextContent("2024100101");
  expect(table).toHaveTextContent("张宇");
  expect(table).toHaveTextContent("2024-09-28 09:15");
  expect(table).toHaveTextContent("赵雨");
  expect(table).toHaveTextContent("98");
  expect(within(table).getAllByRole("button", { name: "查看答卷" })).toHaveLength(4);
  expect(within(table).getAllByRole("button", { name: "重发邀请码" })).toHaveLength(2);

  expect(within(panel).getByText("共 128 条")).toBeInTheDocument();
  expect(within(panel).getByRole("button", { name: "第 1 页" })).toHaveAttribute("aria-current", "page");
  expectSelectText(within(panel).getByLabelText("考生分页"), "1 / 4 页");
});

test("考试详情考生管理标签页接入真实考生列表接口", async () => {
  const user = userEvent.setup();
  const examDetailApi = createExamDetailApiDouble();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  expect(await screen.findByText("后端考生")).toBeInTheDocument();
  const panel = screen.getByRole("tabpanel", { name: "考生管理" });
  expect(within(panel).getByText("2024100999")).toBeInTheDocument();
  expect(within(panel).getByText("后端空间")).toBeInTheDocument();
  expect(within(panel).getByText("91")).toBeInTheDocument();
  expect(within(panel).getByText("共 1 条")).toBeInTheDocument();
  expect(examDetailApi.getCandidates).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    status: undefined,
    keyword: undefined,
    page: 1,
    pageSize: 20,
  });
  expect(screen.queryByText("李明")).not.toBeInTheDocument();
});

test("考试详情考生分页下拉会触发真实翻页请求", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getCandidates: vi.fn(async (input) => ({
      exam: examDetailExam(),
      items: [{
        userID: 20 + input.page,
        username: `20241009${input.page}${input.page}`,
        realName: input.page === 2 ? "第二页考生" : "第一页考生",
        spaceID: 301,
        spaceName: "后端空间",
        status: "submitted" as const,
        startedAt: 1779792000000,
        submittedAt: 1779795600000,
        totalScore: "91",
        attemptCount: 1,
        currentAttemptID: null,
        resultAttemptID: 1800 + input.page,
        sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "后端空间" }],
      }],
      page: input.page,
      pageSize: input.pageSize,
      total: 45,
      permissions: examDetailPermissions(),
    })),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));
  expect(await screen.findByText("第一页考生")).toBeInTheDocument();

  await user.click(screen.getByRole("combobox", { name: "考生分页" }));
  await user.click(await screen.findByRole("option", { name: "2 / 3 页" }));

  expect(await screen.findByText("第二页考生")).toBeInTheDocument();
  expect(examDetailApi.getCandidates).toHaveBeenLastCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    status: undefined,
    keyword: undefined,
    page: 2,
    pageSize: 20,
  });
  expectSelectText(screen.getByLabelText("考生分页"), "2 / 3 页");
});

test("考试详情考生管理的班级筛选会收窄当前表格和发送邀请码目标", async () => {
  const user = userEvent.setup();
  const resendInvitations = vi.fn(async () => ({
    sentCount: 1,
    skippedCount: 0,
    inviteCode: "PM8888",
    permissions: examDetailPermissions(),
  }));
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getCandidates: vi.fn(async () => ({
      exam: examDetailExam(),
      items: [
        {
          userID: 21,
          username: "2024100101",
          realName: "一班考生",
          spaceID: 301,
          spaceName: "高一1班",
          status: "not_started" as const,
          startedAt: null,
          submittedAt: null,
          totalScore: "",
          attemptCount: 0,
          currentAttemptID: null,
          resultAttemptID: null,
          sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "高一1班" }],
        },
        {
          userID: 22,
          username: "2024100102",
          realName: "二班考生",
          spaceID: 302,
          spaceName: "高一2班",
          status: "not_started" as const,
          startedAt: null,
          submittedAt: null,
          totalScore: "",
          attemptCount: 0,
          currentAttemptID: null,
          resultAttemptID: null,
          sourceTargets: [{ targetType: "space" as const, targetID: 302, spaceID: 302, spaceName: "高一2班" }],
        },
      ],
      page: 1,
      pageSize: 20,
      total: 2,
      permissions: examDetailPermissions(),
    })),
    resendInvitations,
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  const panel = await screen.findByRole("tabpanel", { name: "考生管理" });
  expect(await within(panel).findByText("一班考生")).toBeInTheDocument();
  expect(within(panel).getByText("二班考生")).toBeInTheDocument();

  await user.click(within(panel).getByRole("combobox", { name: "班级筛选" }));
  await user.click(await screen.findByRole("option", { name: "高一2班" }));

  expect(await within(panel).findByText("二班考生")).toBeInTheDocument();
  expect(within(panel).queryByText("一班考生")).not.toBeInTheDocument();

  await user.click(within(panel).getByRole("button", { name: "发送邀请码" }));

  expect(resendInvitations).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    userIDs: [22],
  });
});

test("考试详情考生管理加载中不回退示例考生数据", async () => {
	const user = userEvent.setup();
	const examDetailApi: ExamDetailAPI = {
		...createExamDetailApiDouble(),
		getCandidates: vi.fn(() => new Promise<Awaited<ReturnType<ExamDetailAPI["getCandidates"]>>>(() => undefined)),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  const panel = await screen.findByRole("tabpanel", { name: "考生管理" });
  expect(within(panel).getByText("正在加载考生列表")).toBeInTheDocument();
	expect(within(panel).queryByText("李明")).not.toBeInTheDocument();
	expect(within(panel).getByText("共 0 条")).toBeInTheDocument();
});

test("考试详情考生管理翻页失败后不保留旧页可操作数据", async () => {
	const user = userEvent.setup();
	const getCandidates = vi.fn(async (input) => {
		if (input.page === 2) {
			throw new ApiError("考生列表加载失败", 500, 500, {});
		}
		return {
			exam: examDetailExam(),
			items: [{
				userID: 21,
				username: "2024100101",
				realName: "第一页待邀请考生",
				spaceID: 301,
				spaceName: "高一1班",
				status: "not_started" as const,
				startedAt: null,
				submittedAt: null,
				totalScore: "",
				attemptCount: 0,
				currentAttemptID: null,
				resultAttemptID: null,
				sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "高一1班" }],
			}],
			page: 1,
			pageSize: 20,
			total: 40,
			permissions: examDetailPermissions(),
		};
	});
	const resendInvitations = vi.fn(async () => ({
		sentCount: 1,
		skippedCount: 0,
		inviteCode: "PM8888",
		permissions: examDetailPermissions(),
	}));
	const examDetailApi: ExamDetailAPI = {
		...createExamDetailApiDouble(),
		getCandidates,
		resendInvitations,
	};

	render(
		<FeedbackProvider>
			<MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
				<Routes>
					<Route
						path="/exams/:examID"
						element={(
							<PaperPreviewPage
								examDetailApi={examDetailApi}
								paperApi={createUnusedPaperApiDouble()}
								questionApi={createUnusedQuestionApiDouble()}
								tenantID={10}
								spaceID={301}
							/>
						)}
					/>
				</Routes>
			</MemoryRouter>
		</FeedbackProvider>,
	);

	expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
	await user.click(screen.getByRole("tab", { name: "考生管理" }));
	const panel = await screen.findByRole("tabpanel", { name: "考生管理" });
	expect(await within(panel).findByText("第一页待邀请考生")).toBeInTheDocument();

	await user.click(within(panel).getByRole("button", { name: "下一页" }));

	await waitFor(() => {
		expect(getCandidates).toHaveBeenCalledTimes(2);
	});
	await waitFor(() => {
		expect(within(panel).queryByText("第一页待邀请考生")).not.toBeInTheDocument();
	});
	expect(within(panel).getByRole("button", { name: "发送邀请码" })).toBeDisabled();
	await user.click(within(panel).getByRole("button", { name: "发送邀请码" }));
	expect(resendInvitations).not.toHaveBeenCalled();
});

test("考试详情操作日志标签页接入真实操作日志接口", async () => {
	const user = userEvent.setup();
	const examDetailApi = createExamDetailApiDouble();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "操作日志" }));

  const panel = await screen.findByRole("tabpanel", { name: "操作日志" });
  expect(within(panel).getByRole("table", { name: "操作日志列表" })).toHaveTextContent("修改考试设置");
  expect(within(panel).getByText("修改成绩发布配置")).toBeInTheDocument();
  expect(within(panel).getByText("租户管理员")).toBeInTheDocument();
  expect(examDetailApi.getOperationLogs).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    page: 1,
    pageSize: 20,
  });
});

test("考试详情操作日志分页控件会触发真实翻页请求", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getOperationLogs: vi.fn(async (input) => ({
      exam: examDetailExam(),
      items: [{
        id: 4000 + input.page,
        operationType: "update_settings",
        operationTitle: input.page === 2 ? "第二页日志" : "第一页日志",
        operationDetail: input.page === 2 ? "第二页详情" : "第一页详情",
        actorID: 11,
        actorType: "tenant_user",
        actorRole: "tenant_admin",
        operationGroupID: `group-${input.page}`,
        spaceID: null,
        createdAt: 1779795600000 + input.page,
      }],
      page: input.page,
      pageSize: input.pageSize,
      total: 45,
      permissions: examDetailPermissions(),
    })),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "操作日志" }));

  const panel = await screen.findByRole("tabpanel", { name: "操作日志" });
  expect(within(panel).getByText("第一页日志")).toBeInTheDocument();

  await user.click(within(panel).getByRole("button", { name: "下一页" }));

  expect(await within(panel).findByText("第二页日志")).toBeInTheDocument();
  expect(examDetailApi.getOperationLogs).toHaveBeenLastCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    page: 2,
    pageSize: 20,
  });
  expectSelectText(within(panel).getByLabelText("操作日志分页"), "2 / 3 页");
});

test("考试详情操作日志加载失败会清空上一页日志", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getOperationLogs: vi.fn(async (input) => {
      if (input.page === 2) {
        throw new Error("operation logs unavailable");
      }
      return {
        exam: examDetailExam(),
        items: [{
          id: 4001,
          operationType: "update_settings",
          operationTitle: "第一页日志",
          operationDetail: "第一页详情",
          actorID: 11,
          actorType: "tenant_user",
          actorRole: "tenant_admin",
          operationGroupID: "group-1",
          spaceID: null,
          createdAt: 1779795600000,
        }],
        page: input.page,
        pageSize: input.pageSize,
        total: 45,
        permissions: examDetailPermissions(),
      };
    }),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "操作日志" }));

  const panel = await screen.findByRole("tabpanel", { name: "操作日志" });
  expect(within(panel).getByText("第一页日志")).toBeInTheDocument();

  await user.click(within(panel).getByRole("button", { name: "下一页" }));

  await waitFor(() => {
    expect(examDetailApi.getOperationLogs).toHaveBeenCalledTimes(2);
  });
  expect(await within(panel).findByText("暂无操作日志")).toBeInTheDocument();
  expect(within(panel).queryByText("第一页日志")).not.toBeInTheDocument();
});

test("考试设置标签页接入成绩发布配置保存接口", async () => {
  const user = userEvent.setup();
  const preservedScorePublishTime = 1779802800000;
  const updateSettings = vi.fn(async () => ({
    exam: {
      ...examDetailExam(),
      publishMode: "immediate_score",
      scorePublishTime: preservedScorePublishTime,
    },
    saved: true,
    permissions: examDetailPermissions(),
  }));
  const examDetailApi = {
    ...createExamDetailApiDouble(),
    getDetail: vi.fn(async () => ({
      exam: {
        ...examDetailExam(),
        scorePublishTime: preservedScorePublishTime,
      },
      targets: [{ targetType: "space" as const, targetID: 301 }],
      targetSpaceIDs: [301],
      allowedSpaceIDs: [301],
      permissions: examDetailPermissions(),
    })),
    updateSettings,
  } as ExamDetailAPI & { updateSettings: typeof updateSettings };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考试设置" }));

  const panel = screen.getByRole("tabpanel", { name: "考试设置" });
  expect(within(panel).getByRole("region", { name: "考试发布范围" })).toHaveTextContent("发布后不可在此修改");
  expect(within(panel).getByLabelText("成绩发布方式")).toHaveValue("manual_publish");
  await user.selectOptions(within(panel).getByLabelText("成绩发布方式"), "immediate_score");
  await user.click(within(panel).getByRole("button", { name: "保存设置" }));

  expect(updateSettings).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    publishMode: "immediate_score",
    scorePublishTime: preservedScorePublishTime,
  });
  expect(await screen.findByText("考试设置已保存")).toBeInTheDocument();
});

test("考试设置标签页不展示未接入真实字段的公平性静态配置", async () => {
  const user = userEvent.setup();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={createExamDetailApiDouble()}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "考试设置" }));

  const panel = screen.getByRole("tabpanel", { name: "考试设置" });
  expect(within(panel).queryByRole("region", { name: "公平性配置" })).not.toBeInTheDocument();
  expect(within(panel).queryByText("允许切屏提醒：")).not.toBeInTheDocument();
  expect(within(panel).queryByText("题目顺序随机：")).not.toBeInTheDocument();
});

test("考试详情考生管理标签页支持导入考生和重发邀请码", async () => {
  const user = userEvent.setup();
  const getCandidates = vi.fn(async () => ({
    exam: examDetailExam(),
    items: [{
      userID: 21,
      username: "2024100101",
      realName: "待邀请考生",
      spaceID: 301,
      spaceName: "高一1班",
      status: "not_started" as const,
      startedAt: null,
      submittedAt: null,
      totalScore: "",
      attemptCount: 0,
      currentAttemptID: null,
      resultAttemptID: null,
      sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "高一1班" }],
    }],
    page: 1,
    pageSize: 20,
    total: 1,
    permissions: examDetailPermissions(),
  }));
  const importCandidates = vi.fn(async () => ({
    importedCount: 2,
    skippedCount: 0,
    permissions: examDetailPermissions(),
  }));
  const resendInvitations = vi.fn(async () => ({
    sentCount: 1,
    skippedCount: 0,
    inviteCode: "PM8888",
    permissions: examDetailPermissions(),
  }));
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getCandidates,
    importCandidates,
    resendInvitations,
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  const panel = await screen.findByRole("tabpanel", { name: "考生管理" });
  expect(await within(panel).findByText("待邀请考生")).toBeInTheDocument();

  await user.click(within(panel).getByRole("button", { name: "批量导入考生" }));
  await user.type(within(panel).getByLabelText("导入考生用户 ID"), "31, 32");
  await user.click(within(panel).getByRole("button", { name: "确认导入" }));

  expect(importCandidates).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    userIDs: [31, 32],
  });
  expect(await screen.findByText("已导入 2 名考生，跳过 0 名")).toBeInTheDocument();

  await user.click(within(panel).getByRole("button", { name: "重发邀请码" }));

  expect(resendInvitations).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 8,
    spaceID: 301,
    userIDs: [21],
  });
  expect(await screen.findByText("已发送 1 名考生邀请码，跳过 0 名，邀请码 PM8888")).toBeInTheDocument();
  expect(getCandidates).toHaveBeenCalledTimes(3);
});

test("考试详情考生管理标签页按权限禁用导入和邀请操作", async () => {
  const user = userEvent.setup();
  const readonlyPermissions = { ...examDetailPermissions(), canManageCandidates: false };
  const getCandidates = vi.fn(async () => ({
    exam: examDetailExam(),
    items: [{
      userID: 21,
      username: "2024100101",
      realName: "只读考生",
      spaceID: 301,
      spaceName: "高一1班",
      status: "not_started" as const,
      startedAt: null,
      submittedAt: null,
      totalScore: "",
      attemptCount: 0,
      currentAttemptID: null,
      resultAttemptID: null,
      sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "高一1班" }],
    }],
    page: 1,
    pageSize: 20,
    total: 1,
    permissions: readonlyPermissions,
  }));
  const importCandidates = vi.fn(async () => ({
    importedCount: 1,
    skippedCount: 0,
    permissions: readonlyPermissions,
  }));
  const resendInvitations = vi.fn(async () => ({
    sentCount: 1,
    skippedCount: 0,
    inviteCode: "PM8888",
    permissions: readonlyPermissions,
  }));
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getCandidates,
    importCandidates,
    resendInvitations,
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "考生管理" }));

  const panel = await screen.findByRole("tabpanel", { name: "考生管理" });
  expect(await within(panel).findByText("只读考生")).toBeInTheDocument();

  expect(within(panel).getByRole("button", { name: "批量导入考生" })).toBeDisabled();
  expect(within(panel).getByRole("button", { name: "发送邀请码" })).toBeDisabled();
  expect(within(panel).getByRole("button", { name: "重发邀请码" })).toBeDisabled();
  await user.click(within(panel).getByRole("button", { name: "批量导入考生" }));
  await user.click(within(panel).getByRole("button", { name: "发送邀请码" }));
  await user.click(within(panel).getByRole("button", { name: "重发邀请码" }));

  expect(importCandidates).not.toHaveBeenCalled();
  expect(resendInvitations).not.toHaveBeenCalled();
  expect(within(panel).queryByLabelText("导入考生用户 ID")).not.toBeInTheDocument();
});

test("成绩管理标签页展示成绩分析和成绩列表", async () => {
  const user = userEvent.setup();
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={createExamDetailApiDouble()}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "成绩管理" }));

  expect(screen.getByRole("tab", { name: "成绩管理" })).toHaveAttribute("aria-selected", "true");
  const panel = await screen.findByRole("tabpanel", { name: "成绩管理" });
  const stats = within(panel).getByRole("region", { name: "成绩统计" });

  expect(within(stats).getByText("已交卷")).toBeInTheDocument();
  expect(within(stats).getByText("1")).toBeInTheDocument();
  expect(within(stats).getByText("平均分")).toBeInTheDocument();
  expect(within(stats).getByText("最高分")).toBeInTheDocument();
  expect(within(stats).getAllByText("91")).toHaveLength(2);
  expect(within(stats).getByText("及格率")).toBeInTheDocument();
  expect(within(stats).getByText("100%")).toBeInTheDocument();
  expect(within(stats).getByText("待阅主观题")).toBeInTheDocument();
  expect(within(stats).getByText("0")).toBeInTheDocument();
  expect(within(stats).getByText("份")).toBeInTheDocument();

  const distribution = within(panel).getByRole("region", { name: "成绩分布" });
  expect(within(distribution).getByText("90-99")).toBeInTheDocument();
  expect(within(distribution).getByText("1")).toBeInTheDocument();

  const typeAnalysis = within(panel).getByRole("region", { name: "题型得分分析" });
  expect(within(typeAnalysis).getByText("平均得分率")).toBeInTheDocument();
  expect(within(typeAnalysis).getByText("单选题")).toBeInTheDocument();
  expect(within(typeAnalysis).getAllByText("100%").length).toBeGreaterThan(0);

  expect(within(panel).getByRole("button", { name: "配置成绩发布" })).toBeInTheDocument();
  expect(within(panel).getByRole("button", { name: "导出成绩" })).toBeInTheDocument();
  expect(within(panel).getByPlaceholderText("搜索姓名、学号或班级")).toBeInTheDocument();
  expectSelectText(within(panel).getByLabelText("成绩状态筛选"), "全部成绩状态");

  const table = within(panel).getByRole("table", { name: "成绩列表" });
  expect(table).toHaveTextContent("排名");
  expect(table).toHaveTextContent("客观题");
  expect(table).toHaveTextContent("主观题");
  expect(table).toHaveTextContent("总分");
  expect(table).not.toHaveTextContent("70分");
  expect(table).not.toHaveTextContent("50分");
  expect(table).not.toHaveTextContent("120分");
  expect(table).toHaveTextContent("后端考生");
  expect(table).toHaveTextContent("2024100999");
  expect(table).toHaveTextContent("91");
  expect(table).toHaveTextContent("已发布");
  expect(within(table).getAllByRole("button", { name: "查看成绩" })).toHaveLength(1);
  expect(within(table).getAllByRole("button", { name: "查看答卷" })).toHaveLength(1);
  expectSelectText(within(panel).getByLabelText("成绩分页"), "1 / 1 页");
});

test("成绩管理标签页按后端权限隐藏发布和导出入口", async () => {
  const user = userEvent.setup();
  const readonlyPermissions = {
    ...examDetailPermissions(),
    canExportResults: false,
    canPublishResults: false,
  };
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getDetail: vi.fn(async () => ({
      exam: examDetailExam(),
      targets: [],
      targetSpaceIDs: [301],
      allowedSpaceIDs: [301],
      permissions: readonlyPermissions,
    })),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "成绩管理" }));

  const panel = screen.getByRole("tabpanel", { name: "成绩管理" });
  expect(within(panel).queryByRole("button", { name: "配置成绩发布" })).not.toBeInTheDocument();
  expect(within(panel).queryByRole("button", { name: "导出成绩" })).not.toBeInTheDocument();
  expect(within(panel).getByRole("table", { name: "成绩列表" })).toBeInTheDocument();
});

test("成绩管理配置按钮会切换到考试设置标签页", async () => {
  const user = userEvent.setup();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={createExamDetailApiDouble()}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  const panel = await screen.findByRole("tabpanel", { name: "成绩管理" });
  await user.click(within(panel).getByRole("button", { name: "配置成绩发布" }));

  expect(screen.getByRole("tab", { name: "考试设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tabpanel", { name: "考试设置" })).toBeInTheDocument();
});

test("成绩管理标签页导出成绩按钮调用真实导出接口", async () => {
  const user = userEvent.setup();
  exportResultsMock.mockClear();
  const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);

  render(
    <SessionContext.Provider value={{
      session: {
        selectedSpaceID: 301,
        profileSpaces: [{ id: 1, tenantID: 10, spaceID: 301, role: "space_admin", status: "enabled" }],
        user: { userID: 11, displayName: "阅卷老师", role: "teacher", tenantID: 10 },
      },
      signIn: vi.fn(),
      signOut: vi.fn(),
    }}>
      <FeedbackProvider>
        <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
          <Routes>
            <Route
              path="/exams/:examID"
              element={(
                <PaperPreviewPage
                  examDetailApi={createExamDetailApiDouble()}
                  paperApi={createUnusedPaperApiDouble()}
                  questionApi={createUnusedQuestionApiDouble()}
                  tenantID={10}
                  spaceID={301}
                />
              )}
            />
          </Routes>
        </MemoryRouter>
      </FeedbackProvider>
    </SessionContext.Provider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  await user.click(await screen.findByRole("button", { name: "导出成绩" }));

  await waitFor(() => {
    expect(exportResultsMock).toHaveBeenCalledWith({
      tenantID: 10,
      examID: 8,
      actorID: 11,
      actorRole: "space_admin",
      spaceID: 301,
    });
  });
  expect(anchorClick).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("alert")).toHaveTextContent("已导出 1 条成绩记录");
  anchorClick.mockRestore();
});

test("成绩管理行操作会打开成绩详情并直达指定阅卷记录", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getResults: vi.fn(async () => ({
      exam: examDetailExam(),
      items: [{
        rank: 1,
        attemptID: 1801,
        userID: 21,
        username: "2024100999",
        realName: "后端考生",
        spaceID: 301,
        spaceName: "后端空间",
        objectiveScore: "61",
        subjectiveScore: "0",
        totalScore: "61",
        status: "pending_review" as const,
        submittedAt: 1779795600000,
      }],
      page: 1,
      pageSize: 20,
      total: 1,
      permissions: examDetailPermissions(),
    })),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
        examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
          <Route path="/grading" element={<RouteProbe label="阅卷中心" />} />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  const table = await screen.findByRole("table", { name: "成绩列表" });
  expect(within(table).getByRole("button", { name: "查看成绩" })).toBeDisabled();

  const gradingButton = within(table).getByRole("button", { name: "去阅卷" });
  expect(gradingButton).toBeEnabled();
  await user.click(gradingButton);

  expect(await screen.findByText("阅卷中心")).toBeInTheDocument();
  expect(screen.getByText("/grading?exam_id=8&space_id=301&attempt_id=1801")).toBeInTheDocument();
});

test("成绩管理查看成绩会打开当前记录的成绩详情", async () => {
  const user = userEvent.setup();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={createExamDetailApiDouble()}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  const table = await screen.findByRole("table", { name: "成绩列表" });
  await user.click(within(table).getByRole("button", { name: "查看成绩" }));

  expect(screen.getByRole("dialog", { name: "成绩详情" })).toHaveTextContent("后端考生");
  expect(screen.getByRole("dialog", { name: "成绩详情" })).toHaveTextContent("总分：91");
});

test("成绩管理和考生管理都能查看真实答卷详情", async () => {
  const user = userEvent.setup();
  const getAnswerSheet = vi.fn(async () => ({
    exam: examDetailExam(),
    attempt: {
      attemptID: 1801,
      examID: 8,
      userID: 21,
      username: "2024100999",
      realName: "后端考生",
      objectiveScore: "61",
      subjectiveScore: "30",
      totalScore: "91",
      submittedAt: 1779795600000,
    },
    items: [{
      attemptQuestionID: 1901,
      sectionID: 11,
      questionID: 201,
      sortOrder: 1,
      sectionSnapshot: "一、单项选择题",
      questionSnapshot: "服务端预览题干",
      questionType: "single",
      questionTitle: "服务端预览题干",
      optionSnapshot: "[\"A. 选项一\",\"B. 选项二\"]",
      correctAnswerSnapshot: "A",
      score: "2",
      answerContent: "A",
      answerScore: "2",
      gradingStatus: "auto",
      graderComment: "",
      gradedBy: 0,
      gradedAt: null,
    }],
    permissions: examDetailPermissions(),
  }));
  const examDetailApi = {
    ...createExamDetailApiDouble(),
    getAnswerSheet,
  } as ExamDetailAPI;

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  const resultsTable = await screen.findByRole("table", { name: "成绩列表" });
  await user.click(within(resultsTable).getByRole("button", { name: "查看答卷" }));

  await waitFor(() => {
    expect(getAnswerSheet).toHaveBeenCalledWith({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      attemptID: 1801,
    });
  });
  expect(screen.getByRole("dialog", { name: "答卷详情" })).toHaveTextContent("服务端预览题干");
  expect(screen.getByRole("dialog", { name: "答卷详情" })).toHaveTextContent("标准答案：A");

  await user.click(screen.getByRole("button", { name: "关闭答卷详情" }));
  await user.click(screen.getByRole("tab", { name: "考生管理" }));
  const candidatesTable = await screen.findByRole("table", { name: "考生列表" });
  await user.click(within(candidatesTable).getByRole("button", { name: "查看答卷" }));

  await waitFor(() => {
    expect(getAnswerSheet).toHaveBeenLastCalledWith({
      tenantID: 10,
      examID: 8,
      spaceID: 301,
      attemptID: 1801,
    });
  });
});

test("考试详情成绩管理加载中不回退示例成绩数据", async () => {
  const user = userEvent.setup();
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getResultsSummary: vi.fn(() => new Promise<Awaited<ReturnType<ExamDetailAPI["getResultsSummary"]>>>(() => undefined)),
    getResults: vi.fn(() => new Promise<Awaited<ReturnType<ExamDetailAPI["getResults"]>>>(() => undefined)),
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "成绩管理" }));

  const panel = await screen.findByRole("tabpanel", { name: "成绩管理" });
  expect(within(panel).getByText("正在加载成绩列表")).toBeInTheDocument();
  expect(within(panel).queryByText("张子涵")).not.toBeInTheDocument();
  expect(within(panel).queryByText("86.5")).not.toBeInTheDocument();
});

test("考试详情成绩管理翻页失败后不保留旧页成绩数据", async () => {
  const user = userEvent.setup();
  const getResults = vi.fn(async (input) => {
    if (input.page === 2) {
      throw new ApiError("成绩列表加载失败", 500, 500, {});
    }
    return {
      exam: examDetailExam(),
      items: [{
        rank: 1,
        attemptID: 1801,
        userID: 21,
        username: "2024100999",
        realName: "第一页成绩考生",
        spaceID: 301,
        spaceName: "后端空间",
        objectiveScore: "88.5",
        subjectiveScore: "0",
        totalScore: "88.5",
        status: "published" as const,
        submittedAt: 1779795600000,
      }],
      page: 1,
      pageSize: 20,
      total: 40,
      permissions: examDetailPermissions(),
    };
  });
  const examDetailApi: ExamDetailAPI = {
    ...createExamDetailApiDouble(),
    getResultsSummary: vi.fn(async () => ({
      exam: examDetailExam(),
      stats: {
        submitted: 1,
        averageScore: "88.5",
        highestScore: "88.5",
        passRate: "100%",
        pendingSubjective: 0,
      },
      scoreDistribution: [{ label: "80-89", count: 1 }],
      questionTypeRates: [{ questionType: "single", questionTypeLabel: "单选题", averageRate: 100 }],
      permissions: examDetailPermissions(),
    })),
    getResults,
  };

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/exams/8?space_id=301"]}>
        <Routes>
          <Route
            path="/exams/:examID"
            element={(
              <PaperPreviewPage
                examDetailApi={examDetailApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "后端考试详情" })).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "成绩管理" }));
  const panel = await screen.findByRole("tabpanel", { name: "成绩管理" });
  expect(await within(panel).findByText("第一页成绩考生")).toBeInTheDocument();
  expect(within(panel).getAllByText("88.5").length).toBeGreaterThan(0);

  await user.click(within(panel).getByRole("button", { name: "下一页" }));

  await waitFor(() => {
    expect(getResults).toHaveBeenCalledTimes(2);
  });
  await waitFor(() => {
    expect(within(panel).queryByText("第一页成绩考生")).not.toBeInTheDocument();
  });
  expect(within(panel).queryByText("88.5")).not.toBeInTheDocument();
});

function createPaperApiDouble(): PaperAPI {
  const papers: PaperRow[] = [{
    id: 100,
    tenantID: 10,
    spaceID: 301,
    name: "高一数学月考（2024-09）",
    description: "函数与集合阶段测评",
    durationMinutes: 120,
    gradeLevel: "高一",
    totalScore: "120",
    buildMode: "manual",
    status: "enabled",
    createdAt: new Date("2026-06-03T09:30:00+08:00").getTime(),
    creatorName: "teacher.exam",
  }];
  const sections: PaperSectionRow[] = [
    {
      id: 11,
      tenantID: 10,
      paperID: 100,
      sortOrder: 1,
      name: "一、选择题",
      questionType: "single",
      instructions: "单选题（共15题，45分）\n多选题（共5题，15分）",
      totalScore: "60",
      questionCount: 20,
    },
    {
      id: 12,
      tenantID: 10,
      paperID: 100,
      sortOrder: 2,
      name: "二、填空题",
      questionType: "fill_blank",
      instructions: "",
      totalScore: "20",
      questionCount: 5,
    },
    {
      id: 13,
      tenantID: 10,
      paperID: 100,
      sortOrder: 3,
      name: "三、解答题",
      questionType: "short_text",
      instructions: "",
      totalScore: "30",
      questionCount: 5,
    },
    {
      id: 14,
      tenantID: 10,
      paperID: 100,
      sortOrder: 4,
      name: "四、判断题",
      questionType: "judge",
      instructions: "",
      totalScore: "10",
      questionCount: 3,
    } as PaperSectionRow,
  ];
  const sectionQuestions: ManualQuestionRow[] = [
    {
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 201,
      sortOrder: 1,
      score: "3",
    },
    {
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 202,
      sortOrder: 2,
      score: "3",
    },
  ];

  return {
    listPapers: vi.fn(async () => ({ items: papers })),
    createPaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    updatePaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    enablePaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    disablePaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    deletePaper: vi.fn(async () => undefined),
    listSections: vi.fn(async () => ({ items: sections })),
    createSection: vi.fn(async () => {
      throw new Error("not used");
    }),
    reorderSections: vi.fn(async () => undefined),
    deleteSection: vi.fn(async () => undefined),
    addManualQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    listSectionQuestions: vi.fn(async () => ({ items: sectionQuestions })),
    updateSectionQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    replaceSectionQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    deleteSectionQuestion: vi.fn(async () => undefined),
    listRules: vi.fn(async () => ({ items: [] })),
    createRule: vi.fn(async () => {
      throw new Error("not used");
    }),
    updateRule: vi.fn(async () => {
      throw new Error("not used");
    }),
    deleteRule: vi.fn(async () => undefined),
    generateRuleFixed: vi.fn(async () => {
      throw new Error("not used");
    }),
    precheckRuleLive: vi.fn(async () => {
      throw new Error("not used");
    }),
    updateBuildMode: vi.fn(async () => {
      throw new Error("not used");
    }),
  };
}

function createQuestionApiDouble(): QuestionAPI {
  const questions: QuestionRow[] = [
    {
      id: 201,
      tenantID: 10,
      spaceID: 301,
      type: "single",
      title: "已知集合 A = {x | -2 ≤ x ≤ 4}，B = {x | x > 1}，则 A∩B =（ ）",
      stem: "已知集合 A = {x | -2 ≤ x ≤ 4}，B = {x | x > 1}，则 A∩B =（ ）",
      options: ["{x | -2 ≤ x < 1}", "{x | 1 < x < 4}", "{x | 1 ≤ x ≤ 4}", "{x | -2 ≤ x ≤ 1}"],
      correctOptionIndexes: [1],
      analysis: "交集取公共部分。",
      difficulty: "easy",
      tag: "集合",
      tags: ["集合", "高一"],
      scoreDefault: "3",
      status: "ready",
    },
    {
      id: 202,
      tenantID: 10,
      spaceID: 301,
      type: "single",
      title: "函数 f(x)=x² - 2x + 1 的最小值为（ ）",
      stem: "函数 f(x)=x² - 2x + 1 的最小值为（ ）",
      options: ["-1", "0", "1", "2"],
      correctOptionIndexes: [1],
      analysis: "配方得 (x-1)²。",
      difficulty: "easy",
      tag: "函数",
      tags: ["函数", "高一"],
      scoreDefault: "3",
      status: "ready",
    },
  ];

  return {
    listQuestions: vi.fn(async () => ({
      items: questions,
      page: 1,
      pageSize: 100,
      total: questions.length,
    })),
    getQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    createQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    updateQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    disableQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    enableQuestion: vi.fn(async () => {
      throw new Error("not used");
    }),
    deleteQuestion: vi.fn(async () => undefined),
    importQuestions: vi.fn(async () => ({
      successCount: 0,
      duplicateCount: 0,
      errors: [],
    })),
    startQuestionImportJob: vi.fn(async () => ({ jobID: "job-1" })),
    subscribeQuestionImportJob: vi.fn(() => () => undefined),
  };
}

function createExamDetailApiDouble(): ExamDetailAPI {
  return {
    getDetail: vi.fn(async () => ({
      exam: examDetailExam(),
      targets: [{ targetType: "space" as const, targetID: 301 }],
      targetSpaceIDs: [301],
      allowedSpaceIDs: [301],
      permissions: examDetailPermissions(),
    })),
    getOverview: vi.fn(async () => ({
      exam: examDetailExam(),
      candidateStats: {
        planned: 1,
        joined: 0,
        submitted: 0,
        inProgress: 0,
      },
      questionTypes: [{ questionType: "single", questionCount: 1, totalScore: "3" }],
      recentActivities: [],
      permissions: examDetailPermissions(),
    })),
    getPaperPreview: vi.fn(async () => ({
      exam: examDetailExam(),
      sections: [{
        sectionID: 11,
        sectionName: "一、单项选择题",
        questionType: "single",
        questionCount: 1,
        totalScore: "3",
      }],
      items: [{
        sectionID: 11,
        sectionName: "一、单项选择题",
        questionID: 201,
        questionType: "single",
        title: "后端返回题干",
        score: "3",
        blankCount: 0,
        sortOrder: 1,
        options: [{ id: 1, key: "A", content: "后端选项 A" }],
      }],
      page: 1,
      pageSize: 1,
      total: 1,
      permissions: examDetailPermissions(),
    })),
    getCandidates: vi.fn(async () => ({
      exam: examDetailExam(),
      items: [{
        userID: 21,
        username: "2024100999",
        realName: "后端考生",
        spaceID: 301,
        spaceName: "后端空间",
        status: "submitted" as const,
        startedAt: 1779792000000,
        submittedAt: 1779795600000,
        totalScore: "91",
        attemptCount: 1,
        currentAttemptID: null,
        resultAttemptID: 1801,
        sourceTargets: [{ targetType: "space" as const, targetID: 301, spaceID: 301, spaceName: "后端空间" }],
      }],
      page: 1,
      pageSize: 20,
      total: 1,
      permissions: examDetailPermissions(),
    })),
    importCandidates: vi.fn(async () => ({
      importedCount: 0,
      skippedCount: 0,
      permissions: examDetailPermissions(),
    })),
    resendInvitations: vi.fn(async () => ({
      sentCount: 0,
      skippedCount: 0,
      inviteCode: "PM8888",
      permissions: examDetailPermissions(),
    })),
    getResultsSummary: vi.fn(async () => ({
      exam: examDetailExam(),
      stats: {
        submitted: 1,
        averageScore: "91",
        highestScore: "91",
        passRate: "100%",
        pendingSubjective: 0,
      },
      scoreDistribution: [{ label: "90-99", count: 1 }],
      questionTypeRates: [{ questionType: "single", questionTypeLabel: "单选题", averageRate: 100 }],
      permissions: examDetailPermissions(),
    })),
    getResults: vi.fn(async () => ({
      exam: examDetailExam(),
      items: [{
        rank: 1,
        attemptID: 1801,
        userID: 21,
        username: "2024100999",
        realName: "后端考生",
        spaceID: 301,
        spaceName: "后端空间",
        objectiveScore: "91",
        subjectiveScore: "0",
        totalScore: "91",
        status: "published" as const,
        submittedAt: 1779795600000,
      }],
      page: 1,
      pageSize: 20,
      total: 1,
      permissions: examDetailPermissions(),
    })),
    getAnswerSheet: vi.fn(async () => ({
      exam: examDetailExam(),
      attempt: {
        attemptID: 1801,
        examID: 8,
        userID: 21,
        username: "2024100999",
        realName: "后端考生",
        objectiveScore: "91",
        subjectiveScore: "0",
        totalScore: "91",
        submittedAt: 1779795600000,
      },
      items: [{
        attemptQuestionID: 1901,
        sectionID: 11,
        questionID: 201,
        sortOrder: 1,
        sectionSnapshot: "一、单项选择题",
        questionSnapshot: "后端返回题干",
        questionType: "single",
        questionTitle: "后端返回题干",
        optionSnapshot: "[]",
        correctAnswerSnapshot: "A",
        score: "3",
        answerContent: "A",
        answerScore: "3",
        gradingStatus: "auto",
        graderComment: "",
        gradedBy: 0,
        gradedAt: null,
      }],
      permissions: examDetailPermissions(),
    })),
    getOperationLogs: vi.fn(async () => ({
      exam: examDetailExam(),
      items: [{
        id: 3003,
        operationType: "update_settings",
        operationTitle: "修改考试设置",
        operationDetail: "修改成绩发布配置",
        actorID: 11,
        actorType: "tenant_user",
        actorRole: "tenant_admin",
        operationGroupID: "settings-group-1",
        spaceID: null,
        createdAt: 1779795600000,
      }],
      page: 1,
      pageSize: 20,
      total: 1,
      permissions: examDetailPermissions(),
    })),
    updateSettings: vi.fn(async () => ({
      exam: examDetailExam(),
      saved: true,
    })),
  };
}

function createRejectedExamDetailApiDouble(error: ApiError): ExamDetailAPI {
  return {
    getDetail: vi.fn(async () => {
      throw error;
    }),
    getOverview: vi.fn(async () => {
      throw error;
    }),
    getPaperPreview: vi.fn(async () => {
      throw error;
    }),
    getCandidates: vi.fn(async () => {
      throw error;
    }),
    importCandidates: vi.fn(async () => {
      throw error;
    }),
    resendInvitations: vi.fn(async () => {
      throw error;
    }),
    getResultsSummary: vi.fn(async () => {
      throw error;
    }),
    getResults: vi.fn(async () => {
      throw error;
    }),
    getAnswerSheet: vi.fn(async () => {
      throw error;
    }),
    getOperationLogs: vi.fn(async () => {
      throw error;
    }),
    updateSettings: vi.fn(async () => {
      throw error;
    }),
  };
}

function examDetailExam(): ExamDetailExam {
  return {
    id: 8,
    tenantID: 10,
    paperID: 100,
    name: "后端考试详情",
    startTime: 1779792000000,
    endTime: 1779799200000,
    durationMinutes: 120,
    maxAttempts: 1,
    resultStrategy: "latest",
    publishMode: "manual_publish",
    scorePublishTime: null,
    inviteCode: "PM8888",
    status: "published",
    targetType: "space",
    targetID: 301,
  };
}

function createUnusedPaperApiDouble(): PaperAPI {
  const api = createPaperApiDouble();
  api.listPapers = vi.fn(async () => {
    throw new Error("canonical exam route should not load papers");
  });
  return api;
}

function createUnusedQuestionApiDouble(): QuestionAPI {
  const api = createQuestionApiDouble();
  api.listQuestions = vi.fn(async () => {
    throw new Error("canonical exam route should not load question pool");
  });
  return api;
}

function examDetailPermissions(): ExamDetailPermissions {
  return {
    canViewDetail: true,
    canViewOverview: true,
    canViewPaper: true,
    canViewCandidates: true,
    canManageCandidates: true,
    canViewResults: true,
    canExportResults: true,
    canPublishResults: true,
    canUpdateSettings: true,
    canViewLogs: true,
  };
}

function RouteProbe({ label }: { label: string }) {
  const location = useLocation();
  return (
    <div>
      <strong>{label}</strong>
      <span>{`${location.pathname}${location.search}`}</span>
    </div>
  );
}
