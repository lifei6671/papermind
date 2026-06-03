import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import type { PaperAssemblyAPI, PaperBuildMode } from "../../api/papers";
import { PaperAssemblyPage } from "./PaperAssemblyPage";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

test("组卷页默认展示试卷列表而不是试卷详情", async () => {
  renderPaperAssemblyRoutes(createPaperApiDouble());

  expect(screen.getAllByRole("tab")).toHaveLength(2);
  expect(screen.getByRole("tab", { name: "试卷" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "组卷规则" })).toHaveAttribute("aria-selected", "false");
  expect(screen.queryByRole("tab", { name: "题目" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "新建试卷" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索试卷")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新试卷列表" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "试卷名称" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "策略" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "试卷分数" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "创建时间" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "创建人" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "操作区" })).toBeInTheDocument();
  expect(screen.queryByText("当前试卷")).not.toBeInTheDocument();
  expect(screen.queryByText("一、现代文阅读")).not.toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "组卷模式" })).not.toBeInTheDocument();
  expect(await screen.findByRole("row", { name: /高一语文月考试卷/ })).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "teacher.exam" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "编辑试卷 高一语文月考试卷" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "预览试卷 高一语文月考试卷" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "启用试卷 高一语文月考试卷" })).toBeInTheDocument();
});

test("教师可以新建试卷", async () => {
  const user = userEvent.setup();
  renderPaperAssemblyRoutes(createPaperApiDouble());

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "新建试卷" }));

  expect(await screen.findByText("/papers/new")).toBeInTheDocument();
});

test("教师可以搜索和刷新试卷列表", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  const refreshResult = deferred<Awaited<ReturnType<PaperAssemblyAPI["listPapers"]>>>();
  vi.mocked(api.listPapers).mockReturnValueOnce(refreshResult.promise);

  await user.type(screen.getByLabelText("搜索试卷"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByRole("row", { name: /高一语文月考试卷/ })).not.toBeInTheDocument();

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

test("教师可以从列表进入试卷编辑工作台", async () => {
  const user = userEvent.setup();
  renderPaperAssemblyRoutes(createPaperApiDouble());

  await screen.findByRole("row", { name: /高一语文月考试卷/ });
  await user.click(screen.getByRole("button", { name: "编辑试卷 高一语文月考试卷" }));
  expect(await screen.findByText("/papers/100/edit")).toBeInTheDocument();
});

test("教师可以预览并切换试卷启用状态", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await screen.findByRole("row", { name: /高一语文月考试卷/ });

  await user.click(screen.getByRole("button", { name: "预览试卷 高一语文月考试卷" }));
  expect(screen.getByRole("tab", { name: "组卷规则" })).toHaveAttribute("aria-selected", "true");
  expect(await screen.findByText("当前试卷：高一语文月考试卷")).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "试卷" }));
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

test("rule_fixed 生成会复用现有规则而不是重复追加", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  expect(screen.getByRole("button", { name: "新增大题" })).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "组卷模式" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "生成固定规则试卷" }));

  expect(screen.getByRole("dialog", { name: "rule_fixed 弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("固定规则题量"));
  await user.type(screen.getByLabelText("固定规则题量"), "6");
  await user.selectOptions(screen.getByLabelText("固定规则标签"), "阅读理解");
  await user.click(screen.getByRole("button", { name: "确认生成" }));

  await waitFor(() => {
    expect(api.updateRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      ruleID: 301,
      sectionID: 1,
      sortOrder: 2,
      difficulty: "easy",
      tagIDs: [1],
      questionCount: 6,
      scorePerQuestion: "6",
      shuffleOptions: true,
    }));
    expect(api.generateRuleFixed).toHaveBeenCalledWith({ tenantID: 10, paperID: 100 });
  });
  expect(api.createRule).not.toHaveBeenCalled();
  expect(screen.getByRole("status", { name: "rule-fixed-result" })).toHaveTextContent("已按阅读理解生成 6 道题");
  expect(screen.getByText("待替换低匹配题")).toBeInTheDocument();
});

test("rule_fixed 生成前会清理历史遗留的多余规则", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  vi.mocked(api.listRules).mockResolvedValue({
    items: [
      {
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
      },
      {
        id: 302,
        tenantID: 10,
        paperID: 100,
        sectionID: 2,
        sortOrder: 3,
        difficulty: "medium",
        tagIDs: [2],
        questionCount: 4,
        scorePerQuestion: "5",
        shuffleOptions: false,
      },
    ],
  });
  renderPaperAssemblyRoutes(api);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  await user.click(screen.getByRole("button", { name: "生成固定规则试卷" }));
  await user.clear(screen.getByLabelText("固定规则题量"));
  await user.type(screen.getByLabelText("固定规则题量"), "6");
  await user.click(screen.getByRole("button", { name: "确认生成" }));

  await waitFor(() => {
    expect(api.deleteRule).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      ruleID: 302,
    });
    expect(api.generateRuleFixed).toHaveBeenCalledWith({ tenantID: 10, paperID: 100 });
  });
});

test("rule_live 配置和组卷预检查展示风险提示", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  renderPaperAssemblyRoutes(api);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  await user.click(screen.getByRole("button", { name: "保存 rule_live 规则" }));

  expect(screen.getByRole("dialog", { name: "rule_live 弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("动态规则题量"));
  await user.type(screen.getByLabelText("动态规则题量"), "3");
  await user.selectOptions(screen.getByLabelText("动态规则标签"), "语言文字");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  await waitFor(() => {
    expect(api.createRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      tagIDs: [2],
      questionCount: 3,
      scorePerQuestion: "6",
    }));
  });
  expect(screen.getByRole("status", { name: "rule-live-result" })).toHaveTextContent("语言文字");

  await user.click(screen.getByRole("button", { name: "运行组卷预检查" }));

  await waitFor(() => {
    expect(api.precheckRuleLive).toHaveBeenCalledWith({ tenantID: 10, paperID: 100 });
  });
  expect(screen.getByRole("alert")).toHaveTextContent("预检查通过");
  expect(screen.getByRole("alert")).toHaveTextContent("候选题池 2 道题");
});

test("规则列表支持切换模式和编辑规则", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  let activeBuildMode: PaperBuildMode = "rule_fixed";
  vi.mocked(api.listSections).mockImplementation(async () => ({
    items: activeBuildMode === "rule_live"
      ? [
          {
            id: 1,
            tenantID: 10,
            paperID: 100,
            sortOrder: 1,
            name: "一、实时抽题池",
            questionType: "single",
            totalScore: "15",
            questionCount: 3,
          },
        ]
      : [
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
  }));
  vi.mocked(api.updateBuildMode).mockImplementation(async (input) => {
    activeBuildMode = input.buildMode;
    return {
      id: 100,
      tenantID: input.tenantID,
      name: "高一语文月考试卷",
      totalScore: input.buildMode === "rule_live" ? "15" : "50",
      buildMode: input.buildMode,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    };
  });
  renderPaperAssemblyRoutes(api);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  await user.selectOptions(screen.getByLabelText("组卷模式"), "rule_live");

  await waitFor(() => {
    expect(api.updateBuildMode).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      buildMode: "rule_live",
    });
  });
  expect(screen.getByRole("row", { name: /实时抽题组卷/ })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "编辑规则 301" }));
  await user.clear(screen.getByLabelText("规则题量"));
  await user.type(screen.getByLabelText("规则题量"), "3");
  await user.click(screen.getByRole("button", { name: "确认保存规则" }));

  await waitFor(() => {
    expect(api.updateRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      ruleID: 301,
      sortOrder: 2,
      questionCount: 3,
      shuffleOptions: true,
    }));
  });
});

function renderPaperAssemblyRoutes(api: PaperAssemblyAPI) {
  return render(
    <MemoryRouter initialEntries={["/papers?space_id=301"]}>
      <Routes>
        <Route path="/papers" element={<PaperAssemblyPage api={api} tenantID={10} spaceID={301} />} />
        <Route path="/papers/new" element={<LocationProbe />} />
        <Route path="/papers/:paperID/edit" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div>{location.pathname}</div>;
}

function createPaperApiDouble(): PaperAssemblyAPI {
  return {
    listPapers: vi.fn(async () => ({
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
