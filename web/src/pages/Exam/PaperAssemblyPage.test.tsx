import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import type { ActorRole } from "../../api/grading";
import type { PaperAssemblyAPI } from "../../api/papers";
import { FeedbackProvider } from "../../app/feedback";
import { PaperAssemblyPage } from "./PaperAssemblyPage";

afterEach(() => {
  vi.restoreAllMocks();
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

test("组卷页默认展示试卷列表而不是试卷详情", async () => {
  renderPaperAssemblyRoutes(createPaperApiDouble());

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "试卷" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("tab", { name: "组卷规则" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "题目" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "新建试卷" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索试卷")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新试卷列表" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "试卷名称" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "策略" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "试卷分数" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "创建时间" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "归属空间" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "创建人" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "操作区" })).toBeInTheDocument();
  expect(screen.queryByText("当前试卷")).not.toBeInTheDocument();
  expect(screen.queryByText("一、现代文阅读")).not.toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "组卷模式" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "新增大题" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "生成固定规则试卷" })).not.toBeInTheDocument();
  expect(await screen.findByRole("row", { name: /高一语文月考试卷/ })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "teacher.exam" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "编辑试卷 高一语文月考试卷" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "预览试卷 高一语文月考试卷" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "启用试卷 高一语文月考试卷" })).toBeInTheDocument();
});

test("试卷列表展示公共试卷和空间试卷归属", async () => {
  const api = createPaperApiDouble();
  vi.mocked(api.listPapers).mockResolvedValueOnce({
    items: [
      {
        id: 100,
        tenantID: 10,
        spaceID: 301,
        name: "空间试卷",
        totalScore: "18",
        buildMode: "rule_fixed",
        status: "draft" as const,
        createdAt: new Date("2026-06-03T09:30:00+08:00").getTime(),
        creatorName: "teacher.exam",
      },
      {
        id: 101,
        tenantID: 10,
        name: "公共试卷",
        totalScore: "12",
        buildMode: "manual",
        status: "enabled" as const,
        createdAt: new Date("2026-06-03T10:00:00+08:00").getTime(),
        creatorName: "tenant.admin",
      },
    ],
  });

  renderPaperAssemblyRoutes(api);

  expect(await screen.findByRole("row", { name: /空间试卷/ })).toHaveTextContent("空间 301");
  expect(screen.getByRole("row", { name: /公共试卷/ })).toHaveTextContent("公共试卷");
});

test("教师新建试卷需要先填写基础信息再进入组卷页", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "新建试卷" }));

  const dialog = screen.getByRole("dialog", { name: "新建试卷抽屉" });
  expect(dialog).toBeInTheDocument();
  expect(dialog.closest(".ant-drawer")).toBeInTheDocument();
  expect(document.querySelector(".ant-modal")).not.toBeInTheDocument();
  expect(within(dialog).getByRole("button", { name: "返回" })).toBeInTheDocument();
  expect(within(dialog).getByText("|")).toHaveClass("tenant-resource-drawer__separator");
  expect(within(dialog).getByText("新建试卷")).toHaveClass("tenant-resource-drawer__space-name");
  for (const label of ["试卷名称", "组卷方式", "考试时长", "适用年级"]) {
    const field = screen.getByLabelText(label).closest(".field");
    expect(field).not.toBeNull();
    expect(within(field as HTMLElement).getByText("*")).toHaveClass("required-marker");
  }
  await user.type(screen.getByLabelText("试卷名称"), "高一数学周测");
  await user.selectOptions(screen.getByLabelText("组卷方式"), "rule_fixed");
  await user.clear(screen.getByLabelText("考试时长"));
  await user.type(screen.getByLabelText("考试时长"), "90");
  await user.clear(screen.getByLabelText("适用年级"));
  await user.type(screen.getByLabelText("适用年级"), "高一");
  const descriptionEditor = screen.getByLabelText("试卷说明").closest(".markdown-editor");
  expect(descriptionEditor).not.toBeNull();
  expect(descriptionEditor?.querySelector(".w-md-editor")).toBeInTheDocument();
  expect(within(descriptionEditor as HTMLElement).getByTitle("加粗（Ctrl+B）")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("试卷说明"), {
    target: { value: "## 周测说明\n- 覆盖函数与导数\n- **允许草稿纸**" },
  });
  await waitFor(() => {
    expect(within(descriptionEditor as HTMLElement).getByRole("heading", { name: "周测说明" })).toBeInTheDocument();
  });
  await user.click(screen.getByRole("button", { name: "保存并组卷" }));

  await waitFor(() => {
    expect(api.createPaper).toHaveBeenCalledWith({
      tenantID: 10,
      spaceID: 301,
      name: "高一数学周测",
      description: "## 周测说明\n- 覆盖函数与导数\n- **允许草稿纸**",
      durationMinutes: 90,
      gradeLevel: "高一",
    });
  });
  await waitFor(() => {
    expect(api.updateBuildMode).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 101,
      buildMode: "rule_fixed",
    });
  });
  expect(await screen.findByText("/papers/101/edit")).toBeInTheDocument();
});

test("新建试卷抽屉表单保持紧凑控件高度", () => {
  const css = readFileSync(join(process.cwd(), "src/styles/global.css"), "utf8");
  const formRule = css.match(/\.exam-paper-create-form\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const nativeControlRule = css.match(/\.exam-paper-create-form \.field > input,[\s\S]*?\.exam-paper-create-form \.field > select\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const actionButtonRule = css.match(/\.exam-paper-create-form \.platform-dialog__actions \.primary-button,[\s\S]*?\.exam-paper-create-form \.platform-dialog__actions \.secondary-button\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const editorContentRule = css.match(/\.exam-paper-create-form \.markdown-editor \.w-md-editor-show-live \.w-md-editor-content,[\s\S]*?\.exam-paper-create-form \.markdown-editor \.w-md-editor-show-live \.w-md-editor-text-input\s*\{[\s\S]*?\}/)?.[0] ?? "";

  expect(formRule).toContain("align-content: start");
  expect(nativeControlRule).toContain("height: 38px !important");
  expect(nativeControlRule).toContain("min-height: 38px !important");
  expect(actionButtonRule).toContain("height: 38px");
  expect(actionButtonRule).toContain("min-height: 38px");
  expect(editorContentRule).toContain("min-height: 180px !important");
});

test("教师可以搜索和刷新试卷列表", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });

  await user.type(screen.getByLabelText("搜索试卷"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => expect(screen.queryByRole("row", { name: /高一语文月考试卷/ })).not.toBeInTheDocument());

  const refreshResult = deferred<Awaited<ReturnType<PaperAssemblyAPI["listPapers"]>>>();
  vi.mocked(api.listPapers).mockReturnValueOnce(refreshResult.promise);
  await user.click(screen.getByRole("button", { name: "刷新试卷列表" }));
  expect(screen.getByRole("button", { name: "刷新试卷列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );

  refreshResult.resolve({
    items: [{
      id: 100,
      tenantID: 10,
      name: "高一语文月考试卷",
      totalScore: "0",
      buildMode: "rule_fixed",
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    }],
  });

  expect(await screen.findByRole("row", { name: /高一语文月考试卷/ })).toBeInTheDocument();
});

test("试卷列表支持分页切换", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  const paperRows = createPaperRows(21);
  vi.mocked(api.listPapers).mockImplementation(async (input) => ({
    items: paperRows.slice(((input.page ?? 1) - 1) * (input.pageSize ?? 20), (input.page ?? 1) * (input.pageSize ?? 20)),
    page: input.page ?? 1,
    pageSize: input.pageSize ?? 20,
    total: paperRows.length,
  }));

  renderPaperAssemblyRoutes(api);

  expect(await screen.findByRole("row", { name: /模拟试卷 01/ })).toBeInTheDocument();
  expect(screen.getByRole("row", { name: /模拟试卷 20/ })).toBeInTheDocument();
  expect(screen.queryByRole("row", { name: /模拟试卷 21/ })).not.toBeInTheDocument();
  expect(screen.getByText("共 21 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 2 页")).toBeInTheDocument();
  const pagination = screen.getByRole("navigation", { name: "分页" });
  const pageSize = within(pagination).getByRole("combobox", { name: "每页条数" });
  const antPagination = pagination.querySelector(".ant-pagination");
  expect(antPagination).not.toBeNull();
  expect(pageSize.compareDocumentPosition(antPagination as Element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  await user.click(screen.getByRole("button", { name: "下一页" }));

  expect(await screen.findByRole("row", { name: /模拟试卷 21/ })).toBeInTheDocument();
  expect(screen.queryByRole("row", { name: /模拟试卷 01/ })).not.toBeInTheDocument();
  expect(screen.getByText("第 2 / 2 页")).toBeInTheDocument();
});

test("教师可以从列表进入试卷编辑工作台", async () => {
  const user = userEvent.setup();
  renderPaperAssemblyRoutes(createPaperApiDouble());

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "编辑试卷 高一语文月考试卷" }));
  expect(await screen.findByText("/papers/100/edit")).toBeInTheDocument();
});

test("教师不能启用公共试卷", async () => {
  const api = createPaperApiDouble();
  vi.mocked(api.listPapers).mockResolvedValueOnce({
    items: [{
      id: 100,
      tenantID: 10,
      name: "公共数学试卷",
      totalScore: "18",
      buildMode: "rule_fixed",
      status: "draft" as const,
      createdAt: new Date("2026-06-03T09:30:00+08:00").getTime(),
      creatorName: "tenant.admin",
    }],
  });

  renderPaperAssemblyRoutes(api, { actorRole: "teacher" });

  expect(await screen.findByRole("row", { name: /公共数学试卷/ })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "编辑试卷 公共数学试卷" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "启用试卷 公共数学试卷" })).toBeDisabled();
});

test("已发布试卷列表只能预览不能进入编辑", async () => {
  const api = createPaperApiDouble();
  vi.mocked(api.listPapers).mockResolvedValueOnce({
    items: [{
      id: 100,
      tenantID: 10,
      spaceID: 301,
      name: "已发布数学试卷",
      totalScore: "18",
      buildMode: "rule_fixed",
      status: "enabled" as const,
      createdAt: new Date("2026-06-03T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    }],
  });

  renderPaperAssemblyRoutes(api);

  expect(await screen.findByRole("row", { name: /已发布数学试卷/ })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "编辑试卷 已发布数学试卷" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "预览试卷 已发布数学试卷" })).toBeEnabled();
});

test("教师可以预览并切换试卷启用状态", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
  const { unmount } = renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });

  await user.click(screen.getByRole("button", { name: "预览试卷 高一语文月考试卷" }));
  expect(openSpy).toHaveBeenCalledWith(
    "/papers/100/student-preview?space_id=301",
    "_blank",
    "noopener,noreferrer",
  );
  expect(screen.getByRole("tab", { name: "试卷" })).toHaveAttribute("aria-selected", "true");

  unmount();
  renderPaperAssemblyRoutes(api);
  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "启用试卷 高一语文月考试卷" }));

  await waitFor(() => {
    expect(api.enablePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
    });
  });
  expect(await screen.findByRole("cell", { name: "可用" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "禁用试卷 高一语文期末试卷" }));

  await waitFor(() => {
    expect(api.disablePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
    });
  });
  expect(await screen.findByRole("cell", { name: "已禁用" })).toBeInTheDocument();
});

test("切换试卷启用状态失败时使用 toast 提示并透出后端错误", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  vi.mocked(api.enablePaper).mockRejectedValueOnce(new Error("无权执行当前操作"));
  renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "启用试卷 高一语文月考试卷" }));

  const alert = await screen.findByText("无权执行当前操作");
  expect(alert.closest(".ant-message")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
});

function renderPaperAssemblyRoutes(api: PaperAssemblyAPI, { actorRole }: { actorRole?: ActorRole } = {}) {
  return render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={["/papers?space_id=301"]}>
        <Routes>
          <Route path="/papers" element={<PaperAssemblyPage actorRole={actorRole} api={api} tenantID={10} spaceID={301} />} />
          <Route path="/papers/new" element={<LocationProbe />} />
          <Route path="/papers/:paperID/edit" element={<LocationProbe />} />
          <Route path="/papers/:paperID/preview" element={<LocationProbe />} />
          <Route path="/papers/:paperID/student-preview" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </FeedbackProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div>{location.pathname}</div>;
}

function createPaperApiDouble(): PaperAssemblyAPI {
  const defaultPapers = [{
    id: 100,
    tenantID: 10,
    name: "高一语文月考试卷",
    totalScore: "0",
    buildMode: "rule_fixed" as const,
    status: "draft" as const,
    createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
    creatorName: "teacher.exam",
  }];

  return {
    getPaper: vi.fn(async (input) => defaultPapers.find((paper) => paper.id === input.paperID) ?? defaultPapers[0]),
    listPapers: vi.fn(async (input) => {
      const keyword = input.search?.trim().toLowerCase() ?? "";
      const matchedPapers = keyword
        ? defaultPapers.filter((paper) => [paper.name, paper.creatorName, paper.buildMode, paper.status].some((value) => value.toLowerCase().includes(keyword)))
        : defaultPapers;
      return {
        items: matchedPapers.slice(((input.page ?? 1) - 1) * (input.pageSize ?? 20), (input.page ?? 1) * (input.pageSize ?? 20)),
        page: input.page ?? 1,
        pageSize: input.pageSize ?? 20,
        total: matchedPapers.length,
      };
    }),
    listPublishPaperCandidates: vi.fn(async () => ({
      items: defaultPapers.slice(0, 10),
      page: 1,
      pageSize: 10,
      total: defaultPapers.length,
    })),
    createPaper: vi.fn(async (input) => ({
      id: 101,
      tenantID: input.tenantID,
      ...(input.spaceID === undefined ? {} : { spaceID: input.spaceID }),
      name: input.name,
      description: input.description ?? "",
      totalScore: "0",
      buildMode: "manual",
      status: "draft" as const,
      createdAt: new Date("2026-06-02T10:00:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
    updatePaper: vi.fn(async (input) => ({
      id: input.paperID,
      tenantID: input.tenantID,
      name: input.name,
      description: input.description ?? "",
      totalScore: "0",
      buildMode: "rule_fixed",
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
    enablePaper: vi.fn(async (input) => ({
      id: input.paperID,
      tenantID: input.tenantID,
      name: "高一语文期末试卷",
      description: "文学阅读与语言基础",
      totalScore: "0",
      buildMode: "rule_fixed",
      status: "enabled" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
    disablePaper: vi.fn(async (input) => ({
      id: input.paperID,
      tenantID: input.tenantID,
      name: "高一语文期末试卷",
      description: "文学阅读与语言基础",
      totalScore: "0",
      buildMode: "rule_fixed",
      status: "disabled" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
    deletePaper: vi.fn(async () => undefined),
    listSections: vi.fn(async () => ({
      items: [
        {
          id: 1,
          tenantID: 10,
          paperID: 100,
          sortOrder: 1,
          name: "一、现代文阅读",
          questionType: "single",
          totalScore: "30",
          questionCount: 5,
        },
        {
          id: 2,
          tenantID: 10,
          paperID: 100,
          sortOrder: 2,
          name: "二、语言文字运用",
          questionType: "single",
          totalScore: "20",
          questionCount: 5,
        },
      ],
    })),
    createSection: vi.fn(async (input) => ({
      id: 3,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sortOrder: 3,
      name: input.name,
      questionType: input.questionType,
      totalScore: "0",
      questionCount: 0,
    })),
    reorderSections: vi.fn(async () => undefined),
    deleteSection: vi.fn(async () => undefined),
    addManualQuestion: vi.fn(async (input) => ({
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      questionID: input.questionID,
      sortOrder: 1,
      score: input.score,
    })),
    listSectionQuestions: vi.fn(async () => ({ items: [] })),
    updateSectionQuestion: vi.fn(async (input) => ({
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      questionID: input.questionID,
      sortOrder: input.sortOrder,
      score: input.score,
    })),
    replaceSectionQuestion: vi.fn(async (input) => ({
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      questionID: input.newQuestionID,
      sortOrder: input.sortOrder,
      score: input.score,
    })),
    deleteSectionQuestion: vi.fn(async () => undefined),
    listRules: vi.fn(async () => ({
      items: [{
        id: 301,
        tenantID: 10,
        paperID: 100,
        sectionID: 1,
        sortOrder: 2,
        difficulty: "easy",
        tagIDs: [1],
        questionCount: 2,
        scorePerQuestion: "6",
        shuffleOptions: true,
      }],
    })),
    createRule: vi.fn(async (input) => ({
      id: 301,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      sortOrder: input.sortOrder,
      difficulty: input.difficulty,
      tagIDs: input.tagIDs,
      questionCount: input.questionCount,
      scorePerQuestion: input.scorePerQuestion,
      shuffleOptions: input.shuffleOptions,
    })),
    generateRuleFixed: vi.fn(async (input) => ({
      paperID: input.paperID,
      generated: true,
    })),
    precheckRuleLive: vi.fn(async () => ({
      candidateQuestionIDs: [101, 102],
      candidateCount: 2,
    })),
    updateRule: vi.fn(async (input) => ({
      id: input.ruleID,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      sortOrder: input.sortOrder,
      difficulty: input.difficulty,
      tagIDs: input.tagIDs,
      questionCount: input.questionCount,
      scorePerQuestion: input.scorePerQuestion,
      shuffleOptions: input.shuffleOptions,
    })),
    deleteRule: vi.fn(async () => undefined),
    updateBuildMode: vi.fn(async (input) => ({
      id: 100,
      tenantID: input.tenantID,
      name: "高一语文月考试卷",
      totalScore: "15",
      buildMode: input.buildMode,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
  };
}

function createPaperRows(count: number) {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    tenantID: 10,
    name: `模拟试卷 ${String(index + 1).padStart(2, "0")}`,
    totalScore: String(index + 1),
    buildMode: "rule_fixed" as const,
    status: "draft" as const,
    createdAt: new Date("2026-06-02T09:30:00+08:00").getTime() + index,
    creatorName: "teacher.exam",
  }));
}
