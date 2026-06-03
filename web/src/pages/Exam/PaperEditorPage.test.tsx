import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { FeedbackProvider } from "../../app/feedback";
import type { ManualQuestionRow, PaperAPI, PaperRow, PaperSectionRow } from "../../api/papers";
import type { QuestionAPI } from "../../api/questions";
import { PaperEditRoute } from "./PaperEditRoute";
import { PaperEditorPage } from "./PaperEditorPage";

afterEach(() => {
  vi.useRealTimers();
});

test("新建试卷工作台先展示完整骨架，并要求先保存草稿再开始组卷", async () => {
  const { container } = renderPaperEditorRoutes();

  await waitFor(() => expect(screen.getByText("关于函数 y = 1/x，下列说法正确的是（ ）")).toBeInTheDocument());
  expect(screen.getByRole("tab", { name: "组卷管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("button", { name: "保存草稿" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "生成预览" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "发布试卷" })).toBeDisabled();
  expect(screen.getByRole("heading", { name: "题库筛选" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "已选试题" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新组卷数据" })).toBeInTheDocument();
  expect(screen.queryByRole("textbox", { name: "搜索试卷名称、题库或题型" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "加入试卷 题目 201" })).toBeInTheDocument();
  expect(screen.getByText("请先保存试卷基础信息后再开始组卷")).toBeInTheDocument();
  expect(container.querySelector("select:not([aria-hidden='true'])")).not.toBeInTheDocument();
});

test("题库筛选使用自定义下拉控件过滤候选题", async () => {
  const user = userEvent.setup();
  const { container } = renderPaperEditorRoutes();

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");
  expect(container.querySelector("select:not([aria-hidden='true'])")).not.toBeInTheDocument();

  await user.click(screen.getByRole("combobox", { name: "题型筛选" }));
  await user.click(screen.getByRole("option", { name: "简答题" }));

  expect(screen.getByRole("combobox", { name: "题型筛选" })).toHaveTextContent("简答题");
  expect(screen.getByText("请简述港珠澳大桥建设的意义")).toBeInTheDocument();
  expect(screen.queryByText("关于函数 y = 1/x，下列说法正确的是（ ）")).not.toBeInTheDocument();
});

test("题库候选池会自动拉取所有分页结果", async () => {
  const user = userEvent.setup();
  const questionApi = createQuestionApiDouble();
  const firstPageItems = Array.from({ length: 100 }, (_, index) => ({
    id: 3000 + index,
    tenantID: 10,
    type: "single" as const,
    title: `第一页题目 ${index + 1}`,
    stem: `第一页题目 ${index + 1}`,
    options: ["A", "B", "C", "D"],
    correctOptionIndexes: [0],
    analysis: "用于测试分页拉取。",
    difficulty: "medium" as const,
    tag: "函数",
    tags: ["函数", "高一"],
    scoreDefault: "5",
    status: "ready" as const,
    authorName: "teacher.exam",
    authorRole: "teacher",
    createdAt: new Date("2026-06-01T10:00:00+08:00").getTime(),
  }));
  const lastPageItem = {
    id: 4001,
    tenantID: 10,
    type: "short_text" as const,
    title: "最后一页题目",
    stem: "最后一页题目",
    options: [],
    correctOptionIndexes: [],
    analysis: "用于验证第二页题目也会进入候选池。",
    difficulty: "hard" as const,
    tag: "压轴题",
    tags: ["压轴题", "高二"],
    scoreDefault: "12",
    status: "ready" as const,
    authorName: "teacher.exam",
    authorRole: "teacher",
    createdAt: new Date("2026-06-01T12:00:00+08:00").getTime(),
  };
  questionApi.listQuestions = vi.fn(async (input) => ({
    page: input.page ?? 1,
    pageSize: input.pageSize ?? 100,
    total: 101,
    items: (input.page ?? 1) === 1 ? firstPageItems : [lastPageItem],
  }));

  renderPaperEditorRoutes({ questionApi });

  await screen.findByText("第一页题目 1");
  expect(questionApi.listQuestions).toHaveBeenNthCalledWith(1, { tenantID: 10, spaceID: 301, page: 1, pageSize: 100 });
  expect(questionApi.listQuestions).toHaveBeenNthCalledWith(2, { tenantID: 10, spaceID: 301, page: 2, pageSize: 100 });
  expect(screen.getByText("共 101 条")).toBeInTheDocument();

  await user.click(screen.getByRole("combobox", { name: "题型筛选" }));
  await user.click(screen.getByRole("option", { name: "简答题" }));

  expect(screen.getByText("最后一页题目")).toBeInTheDocument();
});

test("草稿题不会出现在可加入试卷的候选池中", async () => {
  const questionApi = createQuestionApiDouble();
  questionApi.listQuestions = vi.fn(async () => ({
    page: 1,
    pageSize: 20,
    total: 2,
    items: [
      {
        id: 201,
        tenantID: 10,
        type: "single" as const,
        title: "关于函数 y = 1/x，下列说法正确的是（ ）",
        stem: "关于函数 y = 1/x，下列说法正确的是（ ）",
        options: ["A", "B", "C", "D"],
        correctOptionIndexes: [0],
        analysis: "函数图像关于原点中心对称。",
        difficulty: "medium" as const,
        tag: "函数",
        tags: ["函数", "高一"],
        scoreDefault: "5",
        status: "ready" as const,
        authorName: "teacher.exam",
        authorRole: "teacher",
        createdAt: new Date("2026-06-01T10:00:00+08:00").getTime(),
      },
      {
        id: 204,
        tenantID: 10,
        type: "single" as const,
        title: "草稿状态题目不应进入组卷候选池",
        stem: "草稿状态题目不应进入组卷候选池",
        options: ["A", "B"],
        correctOptionIndexes: [0],
        analysis: "草稿题必须手动启用后再组卷。",
        difficulty: "easy" as const,
        tag: "函数",
        tags: ["函数", "高一"],
        scoreDefault: "3",
        status: "draft" as const,
        authorName: "teacher.exam",
        authorRole: "teacher",
        createdAt: new Date("2026-06-01T12:00:00+08:00").getTime(),
      },
    ],
  }));

  renderPaperEditorRoutes({ questionApi });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  expect(screen.queryByText("草稿状态题目不应进入组卷候选池")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "加入试卷 题目 204" })).not.toBeInTheDocument();
});

test("保存草稿后进入真实编辑态，并支持把题目加入试卷", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  const questionApi = createQuestionApiDouble();
  renderPaperEditorRoutes({ paperApi, questionApi });

  await user.type(screen.getByLabelText("试卷名称"), "高一上学期期中数学测试");
  await user.type(screen.getByLabelText("试卷说明"), "函数与导数专项");
  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.createPaper).toHaveBeenCalledWith({
      tenantID: 10,
      spaceID: 301,
      name: "高一上学期期中数学测试",
      description: "函数与导数专项",
      durationMinutes: 120,
      gradeLevel: "高一",
    });
  });
  expect(await screen.findByText("/papers/101/edit")).toBeInTheDocument();
});

test("新建试卷切换组卷方式失败后仍接管已创建草稿，避免重复创建", async () => {
  const user = userEvent.setup();
  const papers = [{
    id: 100,
    tenantID: 10,
    spaceID: 301,
    name: "高一语文月考试卷",
    description: "文学阅读与语言基础",
    durationMinutes: 120,
    gradeLevel: "高一",
    totalScore: "14",
    buildMode: "manual" as const,
    status: "draft" as const,
    createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
    creatorName: "teacher.exam",
  }];
  let nextID = 101;
  let failFirstModeUpdate = true;
  const paperApi = createPaperApiDouble();
  paperApi.listPapers = vi.fn(async () => ({ items: [...papers] }));
  paperApi.createPaper = vi.fn(async (input) => {
    const created = {
      id: nextID++,
      tenantID: input.tenantID,
      spaceID: input.spaceID ?? 301,
      name: input.name,
      description: input.description ?? "",
      durationMinutes: input.durationMinutes,
      gradeLevel: input.gradeLevel ?? "高一",
      totalScore: "0",
      buildMode: "manual" as const,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T10:00:00+08:00").getTime(),
      creatorName: "teacher.exam",
    };
    papers.unshift(created);
    return created;
  });
  paperApi.updatePaper = vi.fn(async (input) => {
    const current = papers.find((item) => item.id === input.paperID);
    if (!current) {
      throw new Error("paper not found");
    }
    const updated = {
      ...current,
      name: input.name,
      description: input.description ?? "",
      durationMinutes: input.durationMinutes ?? current.durationMinutes,
      gradeLevel: input.gradeLevel ?? current.gradeLevel,
    };
    const index = papers.findIndex((item) => item.id === input.paperID);
    papers[index] = updated;
    return updated;
  });
  paperApi.updateBuildMode = vi.fn(async (input) => {
    if (failFirstModeUpdate) {
      failFirstModeUpdate = false;
      throw new Error("切换组卷方式失败");
    }
    const current = papers.find((item) => item.id === input.paperID);
    if (!current) {
      throw new Error("paper not found");
    }
    const updated = { ...current, buildMode: input.buildMode };
    const index = papers.findIndex((item) => item.id === input.paperID);
    papers[index] = updated;
    return updated;
  });

  renderPaperEditorRoutes({ paperApi });

  await user.type(screen.getByLabelText("试卷名称"), "高一上学期期中数学测试");
  await user.click(screen.getByRole("tab", { name: "智能组卷" }));
  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  expect(await screen.findByText(/切换组卷方式失败/)).toBeInTheDocument();
  expect(await screen.findByText("/papers/101/edit")).toBeInTheDocument();
  expect(paperApi.createPaper).toHaveBeenCalledTimes(1);
  expect(paperApi.updateBuildMode).toHaveBeenCalledTimes(1);

  const nameInput = screen.getByLabelText("试卷名称");
  await user.clear(nameInput);
  await user.type(nameInput, "高一上学期期中数学测试（修订）");
  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.updatePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 101,
      name: "高一上学期期中数学测试（修订）",
      description: "",
      durationMinutes: 120,
      gradeLevel: "高一",
    });
  });
  expect(paperApi.createPaper).toHaveBeenCalledTimes(1);
});

test("保存草稿失败时使用 toast 提示并透出后端错误", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  paperApi.createPaper = vi.fn(async () => {
    throw new Error("空间不存在");
  });
  renderPaperEditorRoutes({ paperApi });

  await user.type(screen.getByLabelText("试卷名称"), "高一上学期期中数学测试");
  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  const alert = await screen.findByText("空间不存在");
  expect(alert.closest(".feedback-toast-stack")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-status")).not.toBeInTheDocument();
});

test("智能组卷知识点使用下拉输入选择并保存标签", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble({
    papers: [createPaperRowForTest({ buildMode: "rule_fixed" })],
  });
  paperApi.createRule = vi.fn(async (input) => ({
    id: 501,
    tenantID: input.tenantID,
    paperID: input.paperID,
    sectionID: input.sectionID,
    sortOrder: 1,
    questionType: input.questionType,
    questionCount: input.questionCount,
    scorePerQuestion: input.scorePerQuestion,
    difficulty: input.difficulty,
    tagIDs: input.tagIDs,
    tagNames: input.tagNames,
    questionScope: input.questionScope,
    difficultyPercentages: input.difficultyPercentages,
    prioritizeQuality: input.prioritizeQuality,
    excludeRecentExamQuestions: input.excludeRecentExamQuestions,
    excludeUsedQuestions: input.excludeUsedQuestions,
  }));

  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByRole("group", { name: "难度分布滑块" });
  await user.click(screen.getByLabelText("根据知识点筛选"));
  await user.type(screen.getByRole("textbox", { name: "搜索知识点标签" }), "阅读");
  await user.click(screen.getByRole("option", { name: "阅读理解" }));

  expect(within(screen.getByLabelText("已选知识点标签")).getByText("阅读理解")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.createRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionScope: "tag_filter",
      tagNames: ["阅读理解"],
    }));
  });
});

test("智能组卷难度分布使用分段滑块并保存百分比", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble({
    papers: [createPaperRowForTest({ buildMode: "rule_fixed" })],
  });
  paperApi.createRule = vi.fn(async (input) => ({
    id: 501,
    tenantID: input.tenantID,
    paperID: input.paperID,
    sectionID: input.sectionID,
    sortOrder: 1,
    questionType: input.questionType,
    questionCount: input.questionCount,
    scorePerQuestion: input.scorePerQuestion,
    difficulty: input.difficulty,
    tagIDs: input.tagIDs,
    tagNames: input.tagNames,
    questionScope: input.questionScope,
    difficultyPercentages: input.difficultyPercentages,
    prioritizeQuality: input.prioritizeQuality,
    excludeRecentExamQuestions: input.excludeRecentExamQuestions,
    excludeUsedQuestions: input.excludeUsedQuestions,
  }));

  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  expect(await screen.findByRole("group", { name: "难度分布滑块" })).toBeInTheDocument();
  expect(screen.getByText("较易 30%")).toBeInTheDocument();
  expect(screen.getByText("中等 50%")).toBeInTheDocument();
  expect(screen.getByText("较难 20%")).toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("较易占比边界"), { target: { value: "25" } });
  fireEvent.change(screen.getByLabelText("中等占比边界"), { target: { value: "70" } });

  expect(screen.getByText("较易 25%")).toBeInTheDocument();
  expect(screen.getByText("中等 45%")).toBeInTheDocument();
  expect(screen.getByText("较难 30%")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.createRule).toHaveBeenCalledWith(expect.objectContaining({
      difficultyPercentages: {
        easy: 25,
        medium: 45,
        hard: 30,
      },
    }));
  });
});

test("编辑基础信息后底部草稿状态会联动切换", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();

  await user.clear(screen.getByLabelText("试卷名称"));
  await user.type(screen.getByLabelText("试卷名称"), "高三期末考试（修订）");

  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.updatePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      name: "高三期末考试（修订）",
      description: "文学阅读与语言基础",
      durationMinutes: 120,
      gradeLevel: "高一",
    });
  });
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();
});

test("保存草稿会持久化适用年级", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  const gradeInput = screen.getByLabelText("适用年级");
  await user.clear(gradeInput);
  await user.type(gradeInput, "高二");

  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.updatePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      name: "高一语文月考试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 120,
      gradeLevel: "高二",
    });
  });
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();
});

test("保存适用年级后刷新组卷数据不会回退", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  await user.clear(screen.getByLabelText("适用年级"));
  await user.type(screen.getByLabelText("适用年级"), "高二");
  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => expect(screen.getByText("草稿已保存")).toBeInTheDocument());

  const listCallsBeforeRefresh = vi.mocked(paperApi.listPapers).mock.calls.length;
  const refreshButton = screen.getByRole("button", { name: "刷新组卷数据" });
  await user.click(refreshButton);

  await waitFor(() => expect(paperApi.listPapers).toHaveBeenCalledTimes(listCallsBeforeRefresh + 1));
  await waitFor(() => expect(refreshButton).toBeDisabled());
  await waitFor(() => expect(refreshButton).not.toBeDisabled());
  expect(screen.getByLabelText("适用年级")).toHaveValue("高二");
});

test("保存草稿会持久化考试时长", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  const durationInput = screen.getByLabelText("考试时长");
  await user.clear(durationInput);
  await user.type(durationInput, "150");

  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  await waitFor(() => {
    expect(paperApi.updatePaper).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      name: "高一语文月考试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 150,
      gradeLevel: "高一",
    });
  });
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();
});

test("已存在试卷支持加入试卷、移除试题和调整顺序", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble();
  const questionApi = createQuestionApiDouble();
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi,
  });

  expect(await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "加入试卷 题目 201" }));

  await waitFor(() => {
    expect(paperApi.addManualQuestion).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 201,
      score: "5",
    });
  });
  expect(await screen.findByText("一、单项选择题")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "移除试卷 题目 201" })).toBeInTheDocument();
  expect(screen.getByTitle("1. 关于函数 y = 1/x，下列说法正确的是（ ）")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "移除题目 201" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "加入试卷 题目 202" }));
  await waitFor(() => {
    expect(paperApi.addManualQuestion).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 202,
      score: "5",
    });
  });

  const draggedQuestion = screen.getByRole("button", { name: "拖拽排序题目 202" });
  const questionDropTarget = screen.getByTitle("1. 关于函数 y = 1/x，下列说法正确的是（ ）").closest<HTMLElement>("tr");
  if (questionDropTarget === null) {
    throw new Error("missing question row drop target");
  }
  fireEvent.dragStart(draggedQuestion);
  fireEvent.dragOver(questionDropTarget);
  fireEvent.drop(questionDropTarget);
  fireEvent.dragEnd(draggedQuestion);

  expect(paperApi.updateSectionQuestion).not.toHaveBeenCalled();
  expect(screen.getByTitle("1. 已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）")).toBeInTheDocument();
  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "移除题目 201" }));
  await waitFor(() => {
    expect(paperApi.deleteSectionQuestion).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 201,
    });
  });
  expect(screen.getByRole("button", { name: "加入试卷 题目 201" })).toBeInTheDocument();
});

test("rule_live 试卷在编辑页禁止手工维护试题", async () => {
  const paperApi = createPaperApiDouble({
    papers: [{
      id: 100,
      tenantID: 10,
      spaceID: 301,
      name: "高一语文月考试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 120,
      gradeLevel: "高一",
      totalScore: "14",
      buildMode: "rule_live" as const,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    }],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "5" },
    ],
  });
  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  expect(screen.getByRole("button", { name: "加入试卷 题目 202" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "拖拽排序题目 201" })).toBeDisabled();
  expect(screen.getByRole("spinbutton", { name: "题目 201 分值" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "移除题目 201" })).toBeDisabled();
  expect(screen.getByText("rule_live 试卷请到组卷规则页维护题池和分值。")).toBeInTheDocument();
  expect(paperApi.addManualQuestion).not.toHaveBeenCalled();
  expect(paperApi.updateSectionQuestion).not.toHaveBeenCalled();
  expect(paperApi.deleteSectionQuestion).not.toHaveBeenCalled();
});

test("刷新按钮会重新加载当前组卷数据并重新渲染", async () => {
  const paperApi = createPaperApiDouble();
  const questionApi = createQuestionApiDouble();
  const questionState = {
    page: 1,
    pageSize: 20,
    total: 3,
    items: [...(await questionApi.listQuestions({ tenantID: 10, spaceID: 301 })).items],
  };

  questionApi.listQuestions = vi.fn(async () => ({
    page: questionState.page,
    pageSize: questionState.pageSize,
    total: questionState.total,
    items: questionState.items,
  }));

  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi,
  });

  expect(await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）")).toBeInTheDocument();
  questionState.items = questionState.items.map((item) => (
    item.id === 201 ? { ...item, title: "刷新后的题目标题" } : item
  ));
  vi.useFakeTimers();
  const refreshButton = screen.getByRole("button", { name: "刷新组卷数据" });

  await act(async () => {
    fireEvent.click(refreshButton);
  });

  expect(refreshButton).toBeDisabled();
  expect(refreshButton.querySelector(".tenant-refresh-icon")).toHaveClass("tenant-refresh-icon--spinning");

  await act(async () => {
    await vi.advanceTimersByTimeAsync(899);
  });
  expect(refreshButton).toBeDisabled();

  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(screen.getByText("刷新后的题目标题")).toBeInTheDocument();
  expect(refreshButton).not.toBeDisabled();
  expect(paperApi.listPapers).toHaveBeenCalledTimes(2);
  expect(paperApi.listSections).toHaveBeenCalledTimes(2);
  expect(paperApi.listSectionQuestions).toHaveBeenCalledTimes(2);
  expect(questionApi.listQuestions).toHaveBeenCalledTimes(2);
});

test("支持拖拽调整大题顺序并自动重排标题序号", async () => {
  const paperApi = createPaperApiDouble({
    sections: [
      {
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 2 分",
        totalScore: "4",
        questionCount: 2,
      },
      {
        id: 12,
        tenantID: 10,
        paperID: 100,
        sortOrder: 2,
        name: "二、解答题",
        questionType: "short_text",
        instructions: "每题 10 分",
        totalScore: "10",
        questionCount: 1,
      },
    ],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 12, questionID: 203, sortOrder: 1, score: "10" },
    ],
  });
  const { container } = renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("请简述港珠澳大桥建设的意义");

  const dragHandle = screen.getByRole("button", { name: "拖拽排序大题 11" });
  const targetSection = screen.getByRole("button", { name: "拖拽排序大题 12" }).closest<HTMLElement>("section");
  if (targetSection === null) {
    throw new Error("missing section drop target");
  }
  fireEvent.dragStart(dragHandle);
  expect(targetSection.previousElementSibling).toHaveClass("exam-paper-editor__selected-section--dragging");
  fireEvent.dragOver(targetSection);
  expect(targetSection).toHaveClass("exam-paper-editor__selected-section--drop-up");
  fireEvent.drop(targetSection);
  fireEvent.dragEnd(dragHandle);

  const headings = Array.from(container.querySelectorAll(".exam-paper-editor__selected-section-head h3"))
    .map((node) => node.textContent);

  expect(headings).toEqual(["一、解答题", "二、单项选择题"]);
  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();
});

test("大题支持从后往前拖拽排序", async () => {
  const paperApi = createPaperApiDouble({
    sections: [
      {
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 2 分",
        totalScore: "4",
        questionCount: 2,
      },
      {
        id: 12,
        tenantID: 10,
        paperID: 100,
        sortOrder: 2,
        name: "二、解答题",
        questionType: "short_text",
        instructions: "每题 10 分",
        totalScore: "10",
        questionCount: 1,
      },
    ],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 12, questionID: 203, sortOrder: 1, score: "10" },
    ],
  });
  const { container } = renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("请简述港珠澳大桥建设的意义");

  const dragHandle = screen.getByRole("button", { name: "拖拽排序大题 12" });
  const targetSection = screen.getByRole("button", { name: "拖拽排序大题 11" }).closest<HTMLElement>("section");
  if (targetSection === null) {
    throw new Error("missing section drop target");
  }
  fireEvent.dragStart(dragHandle);
  fireEvent.dragOver(targetSection);
  expect(targetSection).toHaveClass("exam-paper-editor__selected-section--drop-down");
  fireEvent.drop(targetSection);
  fireEvent.dragEnd(dragHandle);

  const headings = Array.from(container.querySelectorAll(".exam-paper-editor__selected-section-head h3"))
    .map((node) => node.textContent);

  expect(headings).toEqual(["一、解答题", "二、单项选择题"]);
  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();
});

test("题目前方提供拖拽句柄，并支持同一大题内拖拽排序", async () => {
  const paperApi = createPaperApiDouble({
    sections: [
      {
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 2 分",
        totalScore: "4",
        questionCount: 2,
      },
    ],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
    ],
  });
  const { container } = renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  const selectedSection = container.querySelector<HTMLElement>(".exam-paper-editor__selected-section");
  if (selectedSection === null) {
    throw new Error("missing selected section");
  }
  const rowsBefore = within(selectedSection).getAllByRole("row");
  expect(rowsBefore[1]).toHaveTextContent("1. 关于函数 y = 1/x，下列说法正确的是（ ）");
  expect(rowsBefore[2]).toHaveTextContent("2. 已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）");

  const dragHandle = screen.getByRole("button", { name: "拖拽排序题目 202" });
  const targetRow = screen.getByTitle("1. 关于函数 y = 1/x，下列说法正确的是（ ）").closest<HTMLElement>("tr");
  if (targetRow === null) {
    throw new Error("missing target row");
  }
  fireEvent.dragStart(dragHandle);
  expect(rowsBefore[2]).toHaveClass("exam-paper-editor__selected-row--dragging");
  fireEvent.dragOver(targetRow);
  expect(targetRow).toHaveClass("exam-paper-editor__selected-row--drop-down");
  fireEvent.drop(targetRow);
  fireEvent.dragEnd(dragHandle);

  const rowsAfter = within(selectedSection).getAllByRole("row");
  expect(rowsAfter[1]).toHaveTextContent("1. 已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）");
  expect(rowsAfter[2]).toHaveTextContent("2. 关于函数 y = 1/x，下列说法正确的是（ ）");
  expect(paperApi.updateSectionQuestion).not.toHaveBeenCalled();
});

test("已选题分值支持输入并通过现有接口保存", async () => {
  const user = userEvent.setup();
  const paperApi = createPaperApiDouble({
    sections: [
      {
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 2 分",
        totalScore: "4",
        questionCount: 2,
      },
      {
        id: 12,
        tenantID: 10,
        paperID: 100,
        sortOrder: 2,
        name: "二、解答题",
        questionType: "short_text",
        instructions: "每题 10 分",
        totalScore: "10",
        questionCount: 1,
      },
    ],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 12, questionID: 203, sortOrder: 1, score: "10" },
    ],
  });

  const { container } = renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("请简述港珠澳大桥建设的意义");

  const scoreInput = screen.getByRole("spinbutton", { name: "题目 201 分值" });
  expect(scoreInput).toHaveValue(2);

  await user.clear(scoreInput);
  await user.type(scoreInput, "3");
  await user.tab();

  await waitFor(() => {
    expect(paperApi.updateSectionQuestion).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      sectionID: 11,
      questionID: 201,
      sortOrder: 1,
      score: "3",
    });
  });
  expect(screen.getByRole("spinbutton", { name: "题目 201 分值" })).toHaveValue(3);
  expect(container.querySelector(".exam-paper-editor__panel-head span")).toHaveTextContent("共 3 题 / 总分 15 分");
});

test("保存草稿会持久化题目排序且不修改已有试卷组卷方式", async () => {
  const user = userEvent.setup();
  let persistedSections: PaperSectionRow[] = [
    {
      id: 11,
      tenantID: 10,
      paperID: 100,
      sortOrder: 1,
      name: "一、单项选择题",
      questionType: "single",
      instructions: "每题 2 分",
      totalScore: "4",
      questionCount: 2,
    },
    {
      id: 12,
      tenantID: 10,
      paperID: 100,
      sortOrder: 2,
      name: "二、解答题",
      questionType: "short_text",
      instructions: "每题 10 分",
      totalScore: "10",
      questionCount: 1,
    },
  ];
  const sectionState: ManualQuestionRow[] = [
    { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
    { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
    { tenantID: 10, paperID: 100, sectionID: 12, questionID: 203, sortOrder: 1, score: "10" },
  ];
  const paperApi = createPaperApiDouble({
    sections: persistedSections,
    sectionQuestions: sectionState,
  });
  paperApi.listSections = vi.fn(async () => ({ items: persistedSections }));
  paperApi.reorderSections = vi.fn(async (input) => {
    persistedSections = input.orders
      .map((item: { sectionID: number; sortOrder: number }) => {
        const section = persistedSections.find((current) => current.id === item.sectionID);
        if (section === undefined) {
          throw new Error(`missing section ${item.sectionID}`);
        }
        return { ...section, sortOrder: item.sortOrder };
      })
      .sort((left: PaperSectionRow, right: PaperSectionRow) => left.sortOrder - right.sortOrder);
  });
  let persistedQuestions = [...sectionState];
  paperApi.listSectionQuestions = vi.fn(async () => ({ items: persistedQuestions }));
  paperApi.updateSectionQuestion = vi.fn(async (input) => {
    persistedQuestions = persistedQuestions.map((item) => (
      item.sectionID === input.sectionID && item.questionID === input.questionID
        ? { ...item, sortOrder: input.sortOrder, score: input.score }
        : item
    ));
    return {
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      questionID: input.questionID,
      sortOrder: input.sortOrder,
      score: input.score,
    };
  });

  renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  const sectionDragHandle = screen.getByRole("button", { name: "拖拽排序大题 12" });
  const sectionTarget = screen.getByRole("button", { name: "拖拽排序大题 11" }).closest<HTMLElement>("section");
  if (sectionTarget === null) {
    throw new Error("missing section drop target");
  }
  fireEvent.dragStart(sectionDragHandle);
  fireEvent.dragOver(sectionTarget);
  fireEvent.drop(sectionTarget);
  fireEvent.dragEnd(sectionDragHandle);

  const dragHandle = screen.getByRole("button", { name: "拖拽排序题目 202" });
  const targetRow = screen.getByTitle("1. 关于函数 y = 1/x，下列说法正确的是（ ）").closest<HTMLElement>("tr");
  if (targetRow === null) {
    throw new Error("missing target row");
  }
  fireEvent.dragStart(dragHandle);
  fireEvent.dragOver(targetRow);
  fireEvent.drop(targetRow);
  fireEvent.dragEnd(dragHandle);

  expect(screen.getByText("草稿有未保存调整")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "保存草稿" }));

  expect(screen.getByLabelText("组卷方式")).toHaveTextContent("手动组卷");
  expect(screen.queryByRole("tab", { name: "智能组卷" })).not.toBeInTheDocument();
  expect(paperApi.updateBuildMode).not.toHaveBeenCalled();
  await waitFor(() => {
    expect(paperApi.reorderSections).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      orders: [
        { sectionID: 12, sortOrder: 1 },
        { sectionID: 11, sortOrder: 2 },
      ],
    });
  });
  await waitFor(() => {
    expect(paperApi.updateSectionQuestion).toHaveBeenCalledTimes(2);
  });
  expect(paperApi.updateSectionQuestion).toHaveBeenNthCalledWith(1, {
    tenantID: 10,
    paperID: 100,
    sectionID: 11,
    questionID: 202,
    sortOrder: 1,
    score: "2",
  });
  expect(paperApi.updateSectionQuestion).toHaveBeenNthCalledWith(2, {
    tenantID: 10,
    paperID: 100,
    sectionID: 11,
    questionID: 201,
    sortOrder: 2,
    score: "2",
  });

  vi.useFakeTimers();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "刷新组卷数据" }));
    await vi.advanceTimersByTimeAsync(900);
  });

  const refreshedHeadings = screen.getAllByRole("heading", { level: 3 }).map((node) => node.textContent);
  expect(refreshedHeadings).toEqual(expect.arrayContaining(["一、解答题", "二、单项选择题"]));
  expect(screen.getByTitle("1. 已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）")).toBeInTheDocument();
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();
  expect(screen.queryByText("草稿已部分保存，当前版本暂不支持持久化大题顺序。")).not.toBeInTheDocument();
});

test("已选试题表格提供固定列组，避免分值列挤占来源题库", async () => {
  const paperApi = createPaperApiDouble({
    sections: [
      {
        id: 11,
        tenantID: 10,
        paperID: 100,
        sortOrder: 1,
        name: "一、单项选择题",
        questionType: "single",
        instructions: "每题 2 分",
        totalScore: "4",
        questionCount: 2,
      },
    ],
    sectionQuestions: [
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 201, sortOrder: 1, score: "2" },
      { tenantID: 10, paperID: 100, sectionID: 11, questionID: 202, sortOrder: 2, score: "2" },
    ],
  });
  const { container } = renderPaperEditorRoutes({
    initialEntry: "/papers/100/edit?space_id=301",
    paperApi,
    questionApi: createQuestionApiDouble(),
  });

  await screen.findByText("关于函数 y = 1/x，下列说法正确的是（ ）");

  const columns = Array.from(container.querySelectorAll(".exam-paper-editor__selected-table col"))
    .map((node) => node.className);

  expect(columns).toEqual([
    "exam-paper-editor__selected-col exam-paper-editor__selected-col--question",
    "exam-paper-editor__selected-col exam-paper-editor__selected-col--source",
    "exam-paper-editor__selected-col exam-paper-editor__selected-col--score",
    "exam-paper-editor__selected-col exam-paper-editor__selected-col--actions",
  ]);
});

function renderPaperEditorRoutes({
  initialEntry = "/papers/new?space_id=301",
  paperApi = createPaperApiDouble(),
  questionApi = createQuestionApiDouble(),
}: {
  initialEntry?: string;
  paperApi?: PaperAPI;
  questionApi?: QuestionAPI;
} = {}) {
  return render(
    <FeedbackProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/papers/new"
            element={<PaperEditorPage paperApi={paperApi} questionApi={questionApi} tenantID={10} spaceID={301} />}
          />
          <Route
            path="/papers/:paperID/edit"
            element={<PaperEditRoute paperApi={paperApi} questionApi={questionApi} tenantID={10} spaceID={301} />}
          />
        </Routes>
        <LocationProbe />
      </MemoryRouter>
    </FeedbackProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div>{location.pathname}</div>;
}

function createPaperRowForTest(overrides: Partial<PaperRow> = {}): PaperRow {
  return {
    id: 100,
    tenantID: 10,
    spaceID: 301,
    name: "高一语文月考试卷",
    description: "文学阅读与语言基础",
    durationMinutes: 120,
    gradeLevel: "高一",
    totalScore: "14",
    buildMode: "manual",
    status: "draft",
    createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
    creatorName: "teacher.exam",
    ...overrides,
  };
}

function createPaperApiDouble({
  papers,
  sections,
  sectionQuestions,
}: {
  papers?: PaperRow[];
  sections?: PaperSectionRow[];
  sectionQuestions?: ManualQuestionRow[];
} = {}): PaperAPI {
  const currentPapers = papers ?? [{
    id: 100,
    tenantID: 10,
    spaceID: 301,
    name: "高一语文月考试卷",
    description: "文学阅读与语言基础",
    durationMinutes: 120,
    gradeLevel: "高一",
    totalScore: "14",
    buildMode: "manual" as const,
    status: "draft" as const,
    createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
    creatorName: "teacher.exam",
  }];
  const currentSections = sections ?? [{
    id: 11,
    tenantID: 10,
    paperID: 100,
    sortOrder: 1,
    name: "一、单项选择题",
    questionType: "single",
    instructions: "每题 5 分",
    totalScore: "0",
    questionCount: 0,
  }];
  const currentSectionQuestions = sectionQuestions ?? [];

  return {
    listPapers: vi.fn(async () => ({
      items: currentPapers,
    })),
    createPaper: vi.fn(async (input) => ({
      id: 101,
      tenantID: input.tenantID,
      ...(input.spaceID === undefined ? {} : { spaceID: input.spaceID }),
      name: input.name,
      description: input.description ?? "",
      durationMinutes: input.durationMinutes,
      gradeLevel: input.gradeLevel ?? "高一",
      totalScore: "0",
      buildMode: "manual" as const,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T10:00:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
    updatePaper: vi.fn(async (input) => {
      const currentIndex = currentPapers.findIndex((item) => item.id === input.paperID);
      const currentPaper = currentPapers[currentIndex];
      const updated = {
        id: input.paperID,
        tenantID: input.tenantID,
        spaceID: currentPaper?.spaceID ?? 301,
        name: input.name,
        description: input.description ?? "",
        durationMinutes: input.durationMinutes ?? currentPaper?.durationMinutes ?? 120,
        gradeLevel: input.gradeLevel ?? currentPaper?.gradeLevel ?? "高一",
        totalScore: currentPaper?.totalScore ?? "10",
        buildMode: currentPaper?.buildMode ?? "manual" as const,
        status: currentPaper?.status ?? "draft" as const,
        createdAt: currentPaper?.createdAt ?? new Date("2026-06-02T09:30:00+08:00").getTime(),
        creatorName: currentPaper?.creatorName ?? "teacher.exam",
      };
      if (currentIndex >= 0) {
        currentPapers[currentIndex] = updated;
      }
      return updated;
    }),
    enablePaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    disablePaper: vi.fn(async () => {
      throw new Error("not used");
    }),
    deletePaper: vi.fn(async () => undefined),
    listSections: vi.fn(async (input) => ({
      items: input.paperID === 100 ? currentSections : [],
    })),
    createSection: vi.fn(async (input) => ({
      id: 11,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sortOrder: 1,
      name: input.name,
      questionType: input.questionType,
      instructions: input.instructions,
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
    listSectionQuestions: vi.fn(async () => ({ items: currentSectionQuestions })),
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
    listRules: vi.fn(async () => ({ items: [] })),
    createRule: vi.fn(async () => {
      throw new Error("not used");
    }),
    updateRule: vi.fn(async () => {
      throw new Error("not used");
    }),
    deleteRule: vi.fn(async () => undefined),
    generateRuleFixed: vi.fn(async () => ({ paperID: 100, generated: true })),
    precheckRuleLive: vi.fn(async () => ({ candidateQuestionIDs: [], candidateCount: 0 })),
    updateBuildMode: vi.fn(async (input) => ({
      id: input.paperID,
      tenantID: input.tenantID,
      spaceID: 301,
      name: "高一语文月考试卷",
      description: "文学阅读与语言基础",
      durationMinutes: 120,
      gradeLevel: "高一",
      totalScore: "10",
      buildMode: input.buildMode,
      status: "draft" as const,
      createdAt: new Date("2026-06-02T09:30:00+08:00").getTime(),
      creatorName: "teacher.exam",
    })),
  };
}

function createQuestionApiDouble(): QuestionAPI {
  return {
    listQuestions: vi.fn(async () => ({
      page: 1,
      pageSize: 20,
      total: 3,
      items: [
        {
          id: 201,
          tenantID: 10,
          type: "single" as const,
          title: "关于函数 y = 1/x，下列说法正确的是（ ）",
          stem: "关于函数 y = 1/x，下列说法正确的是（ ）",
          options: ["A", "B", "C", "D"],
          correctOptionIndexes: [0],
          analysis: "函数图像关于原点中心对称。",
          difficulty: "medium" as const,
          tag: "函数",
          tags: ["函数", "高一"],
          scoreDefault: "5",
          status: "ready" as const,
          authorName: "teacher.exam",
          authorRole: "teacher",
          createdAt: new Date("2026-06-01T10:00:00+08:00").getTime(),
        },
        {
          id: 202,
          tenantID: 10,
          type: "single" as const,
          title: "已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）",
          stem: "已知函数 f(x)=x²+1，则 f(-1) 的值为（ ）",
          options: ["0", "1", "2", "3"],
          correctOptionIndexes: [2],
          analysis: "代入 x=-1 可得 2。",
          difficulty: "easy" as const,
          tag: "函数",
          tags: ["函数", "高一"],
          scoreDefault: "5",
          status: "ready" as const,
          authorName: "teacher.exam",
          authorRole: "teacher",
          createdAt: new Date("2026-06-01T10:30:00+08:00").getTime(),
        },
        {
          id: 203,
          tenantID: 10,
          type: "short_text" as const,
          title: "请简述港珠澳大桥建设的意义",
          stem: "请简述港珠澳大桥建设的意义",
          options: [],
          correctOptionIndexes: [],
          analysis: "从交通、经济与区域协同角度作答。",
          difficulty: "medium" as const,
          tag: "阅读理解",
          tags: ["阅读理解", "高一"],
          scoreDefault: "10",
          status: "ready" as const,
          authorName: "teacher.exam",
          authorRole: "teacher",
          createdAt: new Date("2026-06-01T11:00:00+08:00").getTime(),
        },
      ],
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
    importQuestions: vi.fn(async () => ({ successCount: 0, errors: [] })),
  };
}
