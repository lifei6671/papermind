import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ExamListResult, ExamManagementAPI, ExamRow } from "../../api/exams";
import type { ManualQuestionRow, PaperAPI } from "../../api/papers";
import type { QuestionAPI } from "../../api/questions";
import { FeedbackProvider } from "../../app/feedback";
import { PaperPreviewRoute, PaperStudentPreviewRoute } from "./PaperPreviewRoute";

vi.mock("./PaperPreviewPage", () => ({
  PaperPreviewPage: ({ paperID, spaceID, tenantID }: { paperID?: number; spaceID?: number; tenantID: number }) => (
    <span data-testid="paper-preview-stub">{JSON.stringify({ paperID, spaceID, tenantID })}</span>
  ),
}));

test("旧试卷预览入口能唯一定位考试时跳转 canonical 考试详情", async () => {
  const examApi = createExamApiDouble([{ items: [
    createExamRow({ id: 8, paperID: 100 }),
    createExamRow({ id: 9, paperID: 101 }),
  ], page: 1, pageSize: 100, total: 2 }]);

  renderRoute(examApi, "/papers/100/preview?space_id=301");

  expect(await screen.findByTestId("location")).toHaveTextContent("/exams/8?space_id=301");
  expect(examApi.listExams).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, page: 1, pageSize: 100 });
});

test("旧试卷预览入口会继续翻页查找不在第一页的关联考试", async () => {
  const examApi = createExamApiDouble([
    {
      items: [createExamRow({ id: 8, paperID: 101 })],
      page: 1,
      pageSize: 100,
      total: 101,
    },
    {
      items: [createExamRow({ id: 109, paperID: 100 })],
      page: 2,
      pageSize: 100,
      total: 101,
    },
  ]);

  renderRoute(examApi, "/papers/100/preview?space_id=301");

  expect(await screen.findByTestId("location")).toHaveTextContent("/exams/109?space_id=301");
  expect(examApi.listExams).toHaveBeenNthCalledWith(1, { tenantID: 10, spaceID: 301, page: 1, pageSize: 100 });
  expect(examApi.listExams).toHaveBeenNthCalledWith(2, { tenantID: 10, spaceID: 301, page: 2, pageSize: 100 });
});

test("旧试卷预览入口找不到关联考试时回退到旧试卷预览", async () => {
  const examApi = createExamApiDouble([{ items: [createExamRow({ id: 9, paperID: 101 })], page: 1, pageSize: 100, total: 1 }]);

  renderRoute(examApi, "/papers/100/preview?space_id=301");

  expect(await screen.findByTestId("paper-preview-stub")).toHaveTextContent("{\"paperID\":100,\"spaceID\":301,\"tenantID\":10}");
});

test("旧试卷预览入口定位失败时不回退旧试卷预览", async () => {
  const examApi = {
    ...createExamApiDouble([]),
    listExams: vi.fn(async () => {
      throw new Error("forbidden");
    }),
  };

  renderRoute(examApi, "/papers/100/preview?space_id=301");

  expect(await screen.findByRole("alert")).toHaveTextContent("考试详情定位失败");
  expect(screen.queryByTestId("paper-preview-stub")).not.toBeInTheDocument();
});

test("旧试卷预览入口匹配多场考试时回退到旧试卷预览避免误进错误考试", async () => {
  const examApi = createExamApiDouble([{ items: [
    createExamRow({ id: 8, paperID: 100 }),
    createExamRow({ id: 9, paperID: 100 }),
  ], page: 1, pageSize: 100, total: 2 }]);

  renderRoute(examApi, "/papers/100/preview?space_id=301");

  expect(await screen.findByTestId("paper-preview-stub")).toHaveTextContent("{\"paperID\":100,\"spaceID\":301,\"tenantID\":10}");
});

test("后台学生视角预览按试卷题目渲染学生答题界面", async () => {
  const paperApi = {
    ...createUnusedPaperApiDouble(),
    listPapers: vi.fn(async () => ({
      items: [createPaperRow({ id: 100, name: "高一语文月考试卷" })],
    })),
    listSections: vi.fn(async () => ({
      items: [{
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 5 分",
        totalScore: "5",
        questionCount: 1,
      }],
    })),
    listSectionQuestions: vi.fn(async () => ({
      items: [{
        tenantID: 10,
        paperID: 100,
        sectionID: 11,
        questionID: 201,
        sortOrder: 1,
        score: "5",
        questionType: "single",
        title: "学生视角预览题干",
        options: ["正确选项", "干扰项"],
        blankCount: 1,
      } satisfies ManualQuestionRow],
    })),
  };
  const questionApi = createUnusedQuestionApiDouble();

  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers/100/student-preview?space_id=301"]}>
        <Routes>
          <Route
            path="/papers/:paperID/student-preview"
            element={<PaperStudentPreviewRoute paperApi={paperApi} questionApi={questionApi} tenantID={10} spaceID={301} />}
          />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );

  expect(await screen.findByRole("heading", { name: "高一语文月考试卷" })).toBeInTheDocument();
  expect(await screen.findByText("学生视角预览题干")).toBeInTheDocument();
  expect(screen.getByText("考试信息与答题卡")).toBeInTheDocument();
  expect(questionApi.getQuestion).not.toHaveBeenCalled();
});

function renderRoute(examApi: ExamManagementAPI, initialEntry: string) {
  render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/papers/:paperID/preview"
            element={(
              <PaperPreviewRoute
                examApi={examApi}
                paperApi={createUnusedPaperApiDouble()}
                questionApi={createUnusedQuestionApiDouble()}
                tenantID={10}
                spaceID={301}
              />
            )}
          />
          <Route path="/exams" element={<LocationProbe />} />
          <Route path="/exams/:examID" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <span data-testid="location">{location.pathname}{location.search}</span>;
}

function createExamApiDouble(responses: ExamListResult[]): ExamManagementAPI {
  let index = 0;
  return {
    listExams: vi.fn(async () => responses[Math.min(index++, responses.length - 1)]),
    publishExam: vi.fn(async () => {
      throw new Error("not used");
    }),
  };
}

function createExamRow({ id, paperID }: { id: number; paperID: number }): ExamRow {
  return {
    id,
    tenantID: 10,
    paperID,
    name: `考试 ${id}`,
    paperName: `试卷 ${paperID}`,
    inviteCode: `PM${id}`,
    target: "空间 301",
    status: "published",
    startAt: "2026-06-04 09:00",
    endAt: "2026-06-04 10:00",
    durationMinutes: 60,
  };
}

function createPaperRow({ id, name }: { id: number; name: string }) {
  return {
    id,
    tenantID: 10,
    spaceID: 301,
    name,
    description: "",
    durationMinutes: 60,
    gradeLevel: "高一",
    totalScore: "5",
    buildMode: "manual",
    status: "draft" as const,
    createdAt: new Date("2026-06-08T09:00:00+08:00").getTime(),
    creatorName: "teacher.exam",
  };
}

function createUnusedPaperApiDouble(): PaperAPI {
  const fail = vi.fn(async () => {
    throw new Error("old paper preview route should resolve exams before loading paper preview");
  });
  return {
    listPapers: fail,
    createPaper: fail,
    updatePaper: fail,
    enablePaper: fail,
    disablePaper: fail,
    deletePaper: fail,
    listSections: fail,
    createSection: fail,
    reorderSections: fail,
    deleteSection: fail,
    addManualQuestion: fail,
    listSectionQuestions: fail,
    updateSectionQuestion: fail,
    replaceSectionQuestion: fail,
    deleteSectionQuestion: fail,
    listRules: fail,
    createRule: fail,
    updateRule: fail,
    deleteRule: fail,
    generateRuleFixed: fail,
    precheckRuleLive: fail,
    updateBuildMode: fail,
  };
}

function createUnusedQuestionApiDouble(): QuestionAPI {
  const fail = vi.fn(async () => {
    throw new Error("old paper preview route should resolve exams before loading question preview");
  });
  return {
    listQuestions: fail,
    getQuestion: fail,
    createQuestion: fail,
    updateQuestion: fail,
    disableQuestion: fail,
    enableQuestion: fail,
    deleteQuestion: fail,
    importQuestions: fail,
    startQuestionImportJob: fail,
    subscribeQuestionImportJob: vi.fn(() => () => undefined),
  };
}
